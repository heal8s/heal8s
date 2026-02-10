package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// AlertsReceived counts the total number of alerts received from Alertmanager
	AlertsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heal8s_alerts_received_total",
			Help: "Total number of alerts received from Alertmanager",
		},
		[]string{"alertname", "severity"},
	)

	// RemediationsCreated counts the total number of Remediation CRs created
	RemediationsCreated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heal8s_remediations_created_total",
			Help: "Total number of Remediation CRs created",
		},
		[]string{"action_type", "target_kind", "mode"},
	)

	// RemediationsSucceeded counts successful remediations
	RemediationsSucceeded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heal8s_remediations_succeeded_total",
			Help: "Total number of successful remediations",
		},
		[]string{"action_type", "target_kind", "mode"},
	)

	// RemediationsFailed counts failed remediations
	RemediationsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heal8s_remediations_failed_total",
			Help: "Total number of failed remediations",
		},
		[]string{"action_type", "target_kind", "reason"},
	)

	// RemediationDuration tracks the time from alert to resolution
	RemediationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "heal8s_remediation_duration_seconds",
			Help:    "Time from remediation creation to resolution",
			Buckets: []float64{10, 30, 60, 120, 300, 600, 1800, 3600},
		},
		[]string{"action_type", "phase"},
	)

	// AlertsSkipped counts alerts that were skipped (deduplicated or filtered)
	AlertsSkipped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heal8s_alerts_skipped_total",
			Help: "Total number of alerts skipped",
		},
		[]string{"alertname", "reason"},
	)

	// RemediationPhaseTransitions tracks phase transitions
	RemediationPhaseTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heal8s_remediation_phase_transitions_total",
			Help: "Total number of remediation phase transitions",
		},
		[]string{"from_phase", "to_phase"},
	)
)

func init() {
	// Register custom metrics with controller-runtime metrics registry
	metrics.Registry.MustRegister(
		AlertsReceived,
		RemediationsCreated,
		RemediationsSucceeded,
		RemediationsFailed,
		RemediationDuration,
		AlertsSkipped,
		RemediationPhaseTransitions,
	)
}
