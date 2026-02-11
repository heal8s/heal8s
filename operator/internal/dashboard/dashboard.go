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

package dashboard

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const maxEvents = 500

// Event represents one dashboard event (alert received, remediation applied, etc.)
type Event struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Alert   string    `json:"alert"`
	Target  string    `json:"target"`
	Action  string    `json:"action"`
	Details string    `json:"details"`
	Phase   string    `json:"phase"`
}

var (
	mu     sync.RWMutex
	events []Event
)

func init() {
	events = make([]Event, 0, maxEvents)
}

// RecordAlertReceived records an alert received (e.g. from webhook).
func RecordAlertReceived(alertName, targetKind, targetName, targetNS, action string) {
	mu.Lock()
	defer mu.Unlock()
	target := targetKind + "/" + targetNS + "/" + targetName
	events = append(events, Event{
		Time:   time.Now().UTC(),
		Type:   "alert_received",
		Alert:  alertName,
		Target: target,
		Action: action,
		Phase:  "—",
	})
	trimEvents()
}

// RecordRemediationApplied records a successful remediation (object changed).
func RecordRemediationApplied(remediationName, targetKind, targetName, targetNS, action, details string) {
	mu.Lock()
	defer mu.Unlock()
	target := targetKind + "/" + targetNS + "/" + targetName
	events = append(events, Event{
		Time:    time.Now().UTC(),
		Type:    "remediation_applied",
		Alert:   remediationName,
		Target:  target,
		Action:  action,
		Details: details,
		Phase:   "Succeeded",
	})
	trimEvents()
}

// RecordRemediationFailed records a failed remediation.
func RecordRemediationFailed(remediationName, targetKind, targetName, targetNS, action, reason string) {
	mu.Lock()
	defer mu.Unlock()
	target := targetKind + "/" + targetNS + "/" + targetName
	events = append(events, Event{
		Time:    time.Now().UTC(),
		Type:    "remediation_failed",
		Alert:   remediationName,
		Target:  target,
		Action:  action,
		Details: reason,
		Phase:   "Failed",
	})
	trimEvents()
}

func trimEvents() {
	if len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
	}
}

// GetEvents returns a copy of events (newest last; reverse for display).
func GetEvents() []Event {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Event, len(events))
	copy(out, events)
	return out
}

// ServeHTTP serves the dashboard UI and API.
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/", "/dashboard", "/dashboard/", "/index.html":
		serveHTML(w)
		return
	case "/api/events":
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serveEventsJSON(w)
		return
	default:
		http.NotFound(w, r)
	}
}

func serveEventsJSON(w http.ResponseWriter) {
	list := GetEvents()
	// Newest first for API
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func serveHTML(w http.ResponseWriter) {
	list := GetEvents()
	// Newest first for display
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(htmlHeader))
	for _, e := range list {
		_, _ = w.Write([]byte(renderRow(e)))
	}
	_, _ = w.Write([]byte(htmlFooter))
}

func renderRow(e Event) string {
	rowClass := ""
	switch e.Type {
	case "remediation_applied":
		rowClass = "success"
	case "remediation_failed":
		rowClass = "failed"
	case "alert_received":
		rowClass = "alert"
	}
	return "<tr class=\"" + rowClass + "\"><td>" + e.Time.Format("2006-01-02 15:04:05") + "</td><td>" + e.Type + "</td><td>" + e.Alert + "</td><td>" + e.Target + "</td><td>" + e.Action + "</td><td>" + e.Details + "</td><td>" + e.Phase + "</td></tr>\n"
}

const htmlHeader = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>heal8s — Events</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 1rem; background: #1a1a2e; color: #eee; }
    h1 { color: #eee; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid #444; padding: 0.5rem 0.75rem; text-align: left; }
    th { background: #16213e; }
    tr.success { background: #0d3320; }
    tr.failed { background: #331a0d; }
    tr.alert { background: #1a1a3e; }
    a { color: #7eb8da; }
  </style>
</head>
<body>
  <h1>heal8s — Events &amp; changes</h1>
  <p>Alerts received and remediations applied (in-memory, newest first).</p>
  <table>
    <thead><tr><th>Time (UTC)</th><th>Type</th><th>Alert / Remediation</th><th>Target</th><th>Action</th><th>Details</th><th>Phase</th></tr></thead>
    <tbody>
`

const htmlFooter = `    </tbody>
  </table>
  <p><a href="/api/events">JSON</a></p>
</body>
</html>
`
