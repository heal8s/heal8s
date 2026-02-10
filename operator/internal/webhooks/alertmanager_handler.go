/*
Copyright 2026 heal8s Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8shealerv1alpha1 "github.com/heal8s/heal8s/operator/api/v1alpha1"
	"github.com/heal8s/heal8s/operator/internal/metrics"
	"github.com/heal8s/heal8s/operator/internal/remediate"
)

// AlertmanagerWebhookPayload represents the Alertmanager webhook payload
type AlertmanagerWebhookPayload struct {
	Version           string            `json:"version"`
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	GroupKey          string            `json:"groupKey"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []AlertPayload    `json:"alerts"`
}

// AlertPayload represents an individual alert in the webhook
type AlertPayload struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// AlertDeduplicator handles alert deduplication
type AlertDeduplicator struct {
	mu   sync.RWMutex
	seen map[string]time.Time
	ttl  time.Duration
}

// NewAlertDeduplicator creates a new deduplicator
func NewAlertDeduplicator(ttl time.Duration) *AlertDeduplicator {
	d := &AlertDeduplicator{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}

	// Start cleanup goroutine
	go d.cleanup()

	return d
}

// ShouldProcess checks if the alert should be processed
func (d *AlertDeduplicator) ShouldProcess(fingerprint string, startsAt time.Time) bool {
	key := fmt.Sprintf("%s:%s", fingerprint, startsAt.Format(time.RFC3339))

	d.mu.Lock()
	defer d.mu.Unlock()

	if lastSeen, ok := d.seen[key]; ok && time.Since(lastSeen) < d.ttl {
		return false
	}

	d.seen[key] = time.Now()
	return true
}

func (d *AlertDeduplicator) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		for key, lastSeen := range d.seen {
			if time.Since(lastSeen) > d.ttl {
				delete(d.seen, key)
			}
		}
		d.mu.Unlock()
	}
}

// AlertmanagerHandler handles Alertmanager webhook requests
type AlertmanagerHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	logger       logr.Logger
	dedup        *AlertDeduplicator
	routerConfig remediate.RouterConfig
}

// NewAlertmanagerHandler creates a new Alertmanager webhook handler
func NewAlertmanagerHandler(client client.Client, scheme *runtime.Scheme, logger logr.Logger) *AlertmanagerHandler {
	return &AlertmanagerHandler{
		client:       client,
		scheme:       scheme,
		logger:       logger,
		dedup:        NewAlertDeduplicator(1 * time.Hour),
		routerConfig: remediate.DefaultRouterConfig(),
	}
}

// HandleWebhook handles incoming Alertmanager webhook requests
func (h *AlertmanagerHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error(err, "Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload AlertmanagerWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error(err, "Failed to parse webhook payload")
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	h.logger.Info("Received Alertmanager webhook",
		"receiver", payload.Receiver,
		"status", payload.Status,
		"alertCount", len(payload.Alerts))

	// Process each alert
	for _, alert := range payload.Alerts {
		alertname := alert.Labels["alertname"]
		severity := alert.Labels["severity"]

		// Track received alert
		metrics.AlertsReceived.WithLabelValues(alertname, severity).Inc()

		// Only process firing alerts
		if alert.Status != "firing" {
			h.logger.Info("Skipping non-firing alert",
				"alertname", alertname,
				"status", alert.Status)
			metrics.AlertsSkipped.WithLabelValues(alertname, "not-firing").Inc()
			continue
		}

		// Check deduplication
		if !h.dedup.ShouldProcess(alert.Fingerprint, alert.StartsAt) {
			h.logger.Info("Skipping duplicate alert",
				"alertname", alertname,
				"fingerprint", alert.Fingerprint)
			metrics.AlertsSkipped.WithLabelValues(alertname, "duplicate").Inc()
			continue
		}

		// Process alert asynchronously
		go h.processAlert(alert, payload)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *AlertmanagerHandler) processAlert(alert AlertPayload, payload AlertmanagerWebhookPayload) {
	ctx := context.Background()
	logger := h.logger.WithValues(
		"alertname", alert.Labels["alertname"],
		"fingerprint", alert.Fingerprint,
	)

	logger.Info("Processing alert")

	// Convert to remediate.Alert
	remAlert := remediate.Alert{
		Labels:      alert.Labels,
		Annotations: alert.Annotations,
		Status:      alert.Status,
		StartsAt:    alert.StartsAt.Format(time.RFC3339),
		Fingerprint: alert.Fingerprint,
	}

	// Route alert to remediation spec
	spec, err := remediate.RouteAlert(remAlert, h.routerConfig)
	if err != nil {
		logger.Error(err, "Failed to route alert")
		return
	}

	// Create Remediation CR name
	remediationName := fmt.Sprintf("rem-%s-%s",
		alert.Labels["alertname"],
		alert.StartsAt.Format("20060102-150405"))

	// Set payload
	payloadJSON, _ := json.Marshal(alert)
	spec.Alert.Payload = string(payloadJSON)
	spec.Alert.AlertID = fmt.Sprintf("%s-%d", alert.Fingerprint, alert.StartsAt.Unix())

	// Create Remediation object
	remediation := &k8shealerv1alpha1.Remediation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      remediationName,
			Namespace: spec.Target.Namespace, // Create in same namespace as target
			Labels: map[string]string{
				"k8s-healer.io/alert":       alert.Labels["alertname"],
				"k8s-healer.io/target":      spec.Target.Name,
				"k8s-healer.io/fingerprint": alert.Fingerprint,
			},
		},
		Spec: *spec,
	}

	// Check if remediation already exists (idempotency)
	existing := &k8shealerv1alpha1.Remediation{}
	err = h.client.Get(ctx, client.ObjectKey{
		Namespace: remediation.Namespace,
		Name:      remediation.Name,
	}, existing)

	if err == nil {
		logger.Info("Remediation already exists, skipping", "name", remediation.Name)
		return
	}

	// Create the Remediation CR
	if err := h.client.Create(ctx, remediation); err != nil {
		logger.Error(err, "Failed to create Remediation CR")
		return
	}

	// Track metrics
	metrics.RemediationsCreated.WithLabelValues(
		string(spec.Action.Type),
		spec.Target.Kind,
		string(spec.Strategy.Mode),
	).Inc()

	logger.Info("Created Remediation CR",
		"name", remediation.Name,
		"namespace", remediation.Namespace,
		"action", spec.Action.Type)
}
