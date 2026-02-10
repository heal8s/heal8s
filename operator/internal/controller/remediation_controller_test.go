package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	k8shealerv1alpha1 "github.com/heal8s/heal8s/operator/api/v1alpha1"
)

func TestRemediationReconciler_IncreaseMemory(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	_ = k8shealerv1alpha1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Create test deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx:latest",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	// Create remediation CR
	remediation := &k8shealerv1alpha1.Remediation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-oom",
			Namespace: "default",
		},
		Spec: k8shealerv1alpha1.RemediationSpec{
			Alert: k8shealerv1alpha1.AlertInfo{
				Name:        "KubePodOOMKilled",
				Fingerprint: "test123",
				Severity:    "critical",
				Source:      "test",
			},
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
			Strategy: k8shealerv1alpha1.Strategy{
				Mode:            k8shealerv1alpha1.StrategyModeDirect,
				RequireApproval: false,
				TTL:             "1h",
			},
		},
		Status: k8shealerv1alpha1.RemediationStatus{
			Phase: k8shealerv1alpha1.RemediationPhaseAnalyzing, // so one Reconcile runs apply (Direct path)
		},
	}

	// Create fake client
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment, remediation).
		WithStatusSubresource(remediation).
		Build()

	// Create reconciler
	r := &RemediationReconciler{
		Client: client,
		Scheme: scheme,
	}

	// Reconcile (phase Analyzing -> handleAnalyzingRemediation applies Direct)
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-oom",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	_, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify remediation status
	updatedRemediation := &k8shealerv1alpha1.Remediation{}
	if err := client.Get(ctx, req.NamespacedName, updatedRemediation); err != nil {
		t.Fatalf("Failed to get remediation: %v", err)
	}

	if updatedRemediation.Status.Phase != k8shealerv1alpha1.RemediationPhaseSucceeded {
		t.Errorf("Expected phase Succeeded, got %s", updatedRemediation.Status.Phase)
	}

	// Verify deployment was updated
	updatedDeployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Name: "test-app", Namespace: "default"}, updatedDeployment); err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}

	memLimit := updatedDeployment.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory]
	expected := resource.MustParse("320Mi")
	if memLimit.Cmp(expected) != 0 {
		t.Errorf("Expected memory limit %s, got %s", expected.String(), memLimit.String())
	}
}

func TestRemediationReconciler_ScaleUp(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	_ = k8shealerv1alpha1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	// Create test deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-app",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr(int32(3)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "web"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "web",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}

	// Create remediation CR
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
			Strategy: k8shealerv1alpha1.Strategy{
				Mode: k8shealerv1alpha1.StrategyModeDirect,
			},
		},
		Status: k8shealerv1alpha1.RemediationStatus{
			Phase: k8shealerv1alpha1.RemediationPhaseAnalyzing,
		},
	}

	// Create fake client
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment, remediation).
		WithStatusSubresource(remediation).
		Build()

	// Create reconciler
	r := &RemediationReconciler{
		Client: client,
		Scheme: scheme,
	}

	// Reconcile (phase Analyzing -> apply Direct)
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-scale",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	_, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify deployment replicas
	updatedDeployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Name: "web-app", Namespace: "default"}, updatedDeployment); err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}

	// 3 replicas + 50% = 4.5 -> 5 replicas
	if *updatedDeployment.Spec.Replicas != 5 && *updatedDeployment.Spec.Replicas != 4 {
		t.Errorf("Expected replicas 4 or 5, got %d", *updatedDeployment.Spec.Replicas)
	}
}

func ptr(i int32) *int32 {
	return &i
}
