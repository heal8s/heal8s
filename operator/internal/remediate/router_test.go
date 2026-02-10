package remediate

import (
	"testing"

	k8shealerv1alpha1 "github.com/heal8s/heal8s/operator/api/v1alpha1"
)

func TestRouteAlert(t *testing.T) {
	config := DefaultRouterConfig()

	tests := []struct {
		name         string
		alert        Alert
		expectError  bool
		expectAction k8shealerv1alpha1.ActionType
	}{
		{
			name: "KubePodOOMKilled alert",
			alert: Alert{
				Labels: map[string]string{
					"alertname":  "KubePodOOMKilled",
					"namespace":  "prod",
					"deployment": "api-service",
					"container":  "api",
					"severity":   "critical",
				},
				Annotations: map[string]string{
					"description": "Pod was OOMKilled",
				},
				Status:      "firing",
				Fingerprint: "abc123",
			},
			expectError:  false,
			expectAction: k8shealerv1alpha1.ActionTypeIncreaseMemory,
		},
		{
			name: "KubeHpaMaxedOut alert",
			alert: Alert{
				Labels: map[string]string{
					"alertname":  "KubeHpaMaxedOut",
					"namespace":  "prod",
					"deployment": "web-service",
					"severity":   "warning",
				},
				Status:      "firing",
				Fingerprint: "def456",
			},
			expectError:  false,
			expectAction: k8shealerv1alpha1.ActionTypeScaleUp,
		},
		{
			name: "Unknown alert",
			alert: Alert{
				Labels: map[string]string{
					"alertname": "UnknownAlert",
					"namespace": "prod",
				},
				Status:      "firing",
				Fingerprint: "xyz789",
			},
			expectError: true,
		},
		{
			name: "Missing alertname",
			alert: Alert{
				Labels: map[string]string{
					"namespace": "prod",
				},
				Status:      "firing",
				Fingerprint: "missing",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := RouteAlert(tt.alert, config)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if spec.Action.Type != tt.expectAction {
				t.Errorf("expected action %s, got %s", tt.expectAction, spec.Action.Type)
			}

			if spec.Alert.Name != tt.alert.Labels["alertname"] {
				t.Errorf("expected alert name %s, got %s", tt.alert.Labels["alertname"], spec.Alert.Name)
			}

			if spec.Target.Namespace != tt.alert.Labels["namespace"] {
				t.Errorf("expected namespace %s, got %s", tt.alert.Labels["namespace"], spec.Target.Namespace)
			}
		})
	}
}

func TestExtractTargetFromAlert(t *testing.T) {
	tests := []struct {
		name        string
		alert       Alert
		expectError bool
		expectKind  string
		expectName  string
		expectNs    string
	}{
		{
			name: "deployment label present",
			alert: Alert{
				Labels: map[string]string{
					"namespace":  "prod",
					"deployment": "api-service",
					"container":  "api",
				},
			},
			expectError: false,
			expectKind:  "Deployment",
			expectName:  "api-service",
			expectNs:    "prod",
		},
		{
			name: "statefulset label present",
			alert: Alert{
				Labels: map[string]string{
					"namespace":   "prod",
					"statefulset": "database",
				},
			},
			expectError: false,
			expectKind:  "StatefulSet",
			expectName:  "database",
			expectNs:    "prod",
		},
		{
			name: "pod label only",
			alert: Alert{
				Labels: map[string]string{
					"namespace": "prod",
					"pod":       "api-service-abc123-xyz",
				},
			},
			expectError: false,
			expectKind:  "Deployment",
			expectNs:    "prod",
		},
		{
			name: "missing namespace",
			alert: Alert{
				Labels: map[string]string{
					"deployment": "api-service",
				},
			},
			expectError: true,
		},
		{
			name: "no resource labels",
			alert: Alert{
				Labels: map[string]string{
					"namespace": "prod",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := extractTargetFromAlert(tt.alert)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if target.Kind != tt.expectKind {
				t.Errorf("expected kind %s, got %s", tt.expectKind, target.Kind)
			}
			if target.Name != tt.expectName && tt.expectName != "" {
				t.Errorf("expected name %s, got %s", tt.expectName, target.Name)
			}
			if target.Namespace != tt.expectNs {
				t.Errorf("expected namespace %s, got %s", tt.expectNs, target.Namespace)
			}
		})
	}
}
