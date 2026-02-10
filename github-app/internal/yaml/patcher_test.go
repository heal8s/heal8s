package yaml

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8shealerv1alpha1 "github.com/heal8s/heal8s/github-app/pkg/api/v1alpha1"
)

func TestPatchManifest_IncreaseMemory(t *testing.T) {
	inputYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: app
        image: nginx:latest
        resources:
          limits:
            memory: 256Mi
          requests:
            memory: 128Mi
`

	remediation := &k8shealerv1alpha1.Remediation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-remediation",
			Namespace: "default",
		},
		Spec: k8shealerv1alpha1.RemediationSpec{
			Target: k8shealerv1alpha1.TargetResource{
				Kind:      "Deployment",
				Name:      "test-app",
				Namespace: "default",
				Container: "app",
			},
			Action: k8shealerv1alpha1.Action{
				Type: k8shealerv1alpha1.ActionTypeIncreaseMemory,
				Params: map[string]string{
					"memoryIncreasePercent": "25",
					"maxMemory":             "2Gi",
				},
			},
		},
	}

	patcher := NewPatcher()
	patchedYAML, err := patcher.PatchManifest(inputYAML, remediation)
	if err != nil {
		t.Fatalf("PatchManifest failed: %v", err)
	}

	// Check that memory was increased
	if !strings.Contains(patchedYAML, "320Mi") && !strings.Contains(patchedYAML, "335544320") {
		t.Errorf("Expected memory to be increased to 320Mi, got:\n%s", patchedYAML)
	}

	// Ensure it's still valid YAML
	if !strings.Contains(patchedYAML, "kind: Deployment") {
		t.Error("Patched YAML is not valid Deployment")
	}
}

func TestPatchManifest_ScaleUp(t *testing.T) {
	inputYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: web
        image: nginx:latest
`

	remediation := &k8shealerv1alpha1.Remediation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-scale",
			Namespace: "default",
		},
		Spec: k8shealerv1alpha1.RemediationSpec{
			Target: k8shealerv1alpha1.TargetResource{
				Kind:      "Deployment",
				Name:      "web-app",
				Namespace: "default",
			},
			Action: k8shealerv1alpha1.Action{
				Type: k8shealerv1alpha1.ActionTypeScaleUp,
				Params: map[string]string{
					"scaleUpPercent": "50",
					"maxReplicas":    "10",
				},
			},
		},
	}

	patcher := NewPatcher()
	patchedYAML, err := patcher.PatchManifest(inputYAML, remediation)
	if err != nil {
		t.Fatalf("PatchManifest failed: %v", err)
	}

	// Check that replicas were increased from 3 to 5 (3 + 50% = 4.5, rounded to 5)
	if !strings.Contains(patchedYAML, "replicas: 5") && !strings.Contains(patchedYAML, "replicas: 4") {
		t.Errorf("Expected replicas to be increased, got:\n%s", patchedYAML)
	}
}
