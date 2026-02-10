package remediate

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCalculateMemoryIncrease(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		percent  int
		max      string
		expected string
	}{
		{
			name:     "25% increase from 256Mi",
			current:  "256Mi",
			percent:  25,
			max:      "2Gi",
			expected: "320Mi",
		},
		{
			name:     "50% increase from 128Mi",
			current:  "128Mi",
			percent:  50,
			max:      "2Gi",
			expected: "192Mi",
		},
		{
			name:     "cap at max memory",
			current:  "1800Mi",
			percent:  25,
			max:      "2Gi",
			expected: "2Gi",
		},
		{
			name:     "small memory gets rounded to 64Mi",
			current:  "100Mi",
			percent:  10,
			max:      "2Gi",
			expected: "128Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := resource.MustParse(tt.current)
			max := resource.MustParse(tt.max)
			expected := resource.MustParse(tt.expected)

			result := CalculateMemoryIncrease(current, tt.percent, max)

			if result.Cmp(expected) != 0 {
				t.Errorf("expected %s, got %s", expected.String(), result.String())
			}
		})
	}
}

func TestApplyIncreaseMemory(t *testing.T) {
	tests := []struct {
		name          string
		deployment    *appsv1.Deployment
		containerName string
		params        map[string]string
		expectError   bool
		validateFn    func(*testing.T, *appsv1.Deployment)
	}{
		{
			name: "increase memory for single container",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "app",
									Image: "nginx",
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
			},
			containerName: "app",
			params: map[string]string{
				"memoryIncreasePercent": "25",
				"maxMemory":             "2Gi",
			},
			expectError: false,
			validateFn: func(t *testing.T, d *appsv1.Deployment) {
				container := d.Spec.Template.Spec.Containers[0]
				memLimit := container.Resources.Limits[corev1.ResourceMemory]
				memRequest := container.Resources.Requests[corev1.ResourceMemory]

				expected := resource.MustParse("320Mi")
				if memLimit.Cmp(expected) != 0 {
					t.Errorf("expected limit %s, got %s", expected.String(), memLimit.String())
				}
				if memRequest.Cmp(expected) != 0 {
					t.Errorf("expected request %s, got %s", expected.String(), memRequest.String())
				}
			},
		},
		{
			name: "auto-select single container",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "app",
									Image: "nginx",
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			containerName: "", // Empty - should auto-select
			params: map[string]string{
				"memoryIncreasePercent": "50",
				"maxMemory":             "1Gi",
			},
			expectError: false,
			validateFn: func(t *testing.T, d *appsv1.Deployment) {
				container := d.Spec.Template.Spec.Containers[0]
				memLimit := container.Resources.Limits[corev1.ResourceMemory]

				expected := resource.MustParse("192Mi")
				if memLimit.Cmp(expected) != 0 {
					t.Errorf("expected limit %s, got %s", expected.String(), memLimit.String())
				}
			},
		},
		{
			name: "container not found",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "app",
									Image: "nginx",
								},
							},
						},
					},
				},
			},
			containerName: "nonexistent",
			params:        map[string]string{},
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ApplyIncreaseMemory(tt.deployment, tt.containerName, tt.params)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && tt.validateFn != nil {
				tt.validateFn(t, tt.deployment)
			}
		})
	}
}
