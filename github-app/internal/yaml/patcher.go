package yaml

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/yaml"

	k8shealerv1alpha1 "github.com/heal8s/heal8s/github-app/pkg/api/v1alpha1"
)

// Patcher handles patching Kubernetes YAML manifests
type Patcher struct {
	scheme *runtime.Scheme
}

// NewPatcher creates a new YAML patcher
func NewPatcher() *Patcher {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	return &Patcher{
		scheme: scheme,
	}
}

// PatchManifest patches a Kubernetes manifest based on the remediation action
func (p *Patcher) PatchManifest(yamlContent string, remediation *k8shealerv1alpha1.Remediation) (string, error) {
	// Decode YAML to object
	obj, err := p.decodeYAML(yamlContent)
	if err != nil {
		return "", fmt.Errorf("failed to decode YAML: %w", err)
	}

	// Apply patch based on action type
	switch remediation.Spec.Action.Type {
	case k8shealerv1alpha1.ActionTypeIncreaseMemory:
		if err := p.patchIncreaseMemory(obj, remediation); err != nil {
			return "", fmt.Errorf("failed to patch memory: %w", err)
		}
	case k8shealerv1alpha1.ActionTypeScaleUp:
		if err := p.patchScaleUp(obj, remediation); err != nil {
			return "", fmt.Errorf("failed to patch scale: %w", err)
		}
	default:
		return "", fmt.Errorf("unsupported action type: %s", remediation.Spec.Action.Type)
	}

	// Encode back to YAML
	patchedYAML, err := p.encodeYAML(obj)
	if err != nil {
		return "", fmt.Errorf("failed to encode YAML: %w", err)
	}

	return patchedYAML, nil
}

func (p *Patcher) decodeYAML(yamlContent string) (runtime.Object, error) {
	decode := serializer.NewCodecFactory(p.scheme).UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (p *Patcher) encodeYAML(obj runtime.Object) (string, error) {
	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(yamlBytes), nil
}

func (p *Patcher) patchIncreaseMemory(obj runtime.Object, remediation *k8shealerv1alpha1.Remediation) error {
	// Parse parameters
	increasePercent := 25
	if val, ok := remediation.Spec.Action.Params["memoryIncreasePercent"]; ok {
		fmt.Sscanf(val, "%d", &increasePercent)
	}

	maxMemory := resource.MustParse("2Gi")
	if val, ok := remediation.Spec.Action.Params["maxMemory"]; ok {
		if parsed, err := resource.ParseQuantity(val); err == nil {
			maxMemory = parsed
		}
	}

	containerName := remediation.Spec.Target.Container

	// Get containers based on object type
	var containers *[]corev1.Container
	switch v := obj.(type) {
	case *appsv1.Deployment:
		containers = &v.Spec.Template.Spec.Containers
	case *appsv1.StatefulSet:
		containers = &v.Spec.Template.Spec.Containers
	case *appsv1.DaemonSet:
		containers = &v.Spec.Template.Spec.Containers
	default:
		return fmt.Errorf("unsupported object type: %T", obj)
	}

	// Find and patch container
	containerIndex := -1
	if containerName == "" && len(*containers) == 1 {
		containerIndex = 0
	} else {
		for i, c := range *containers {
			if c.Name == containerName {
				containerIndex = i
				break
			}
		}
	}

	if containerIndex == -1 {
		return fmt.Errorf("container %s not found", containerName)
	}

	container := &(*containers)[containerIndex]

	// Get current memory
	currentMemory := container.Resources.Limits[corev1.ResourceMemory]
	if currentMemory.IsZero() {
		currentMemory = resource.MustParse("256Mi")
	}

	// Calculate new memory
	multiplier := 1.0 + float64(increasePercent)/100.0
	newMemoryValue := int64(float64(currentMemory.Value()) * multiplier)

	// Round to 64Mi
	roundTo := int64(64 * 1024 * 1024)
	newMemoryValue = ((newMemoryValue + roundTo - 1) / roundTo) * roundTo

	newMemory := *resource.NewQuantity(newMemoryValue, resource.BinarySI)
	if newMemory.Cmp(maxMemory) > 0 {
		newMemory = maxMemory
	}

	// Apply patch
	if container.Resources.Limits == nil {
		container.Resources.Limits = corev1.ResourceList{}
	}
	if container.Resources.Requests == nil {
		container.Resources.Requests = corev1.ResourceList{}
	}

	container.Resources.Limits[corev1.ResourceMemory] = newMemory
	container.Resources.Requests[corev1.ResourceMemory] = newMemory

	return nil
}

func (p *Patcher) patchScaleUp(obj runtime.Object, remediation *k8shealerv1alpha1.Remediation) error {
	scalePercent := 50
	if val, ok := remediation.Spec.Action.Params["scaleUpPercent"]; ok {
		fmt.Sscanf(val, "%d", &scalePercent)
	}

	maxReplicas := int32(10)
	if val, ok := remediation.Spec.Action.Params["maxReplicas"]; ok {
		var tmp int
		fmt.Sscanf(val, "%d", &tmp)
		maxReplicas = int32(tmp)
	}

	var currentReplicas *int32
	switch v := obj.(type) {
	case *appsv1.Deployment:
		currentReplicas = v.Spec.Replicas
	case *appsv1.StatefulSet:
		currentReplicas = v.Spec.Replicas
	default:
		return fmt.Errorf("unsupported object type for scale: %T", obj)
	}

	if currentReplicas == nil {
		one := int32(1)
		currentReplicas = &one
	}

	// Calculate new replicas
	increase := int32(float64(*currentReplicas) * float64(scalePercent) / 100.0)
	if increase < 1 {
		increase = 1
	}

	newReplicas := *currentReplicas + increase
	if newReplicas > maxReplicas {
		newReplicas = maxReplicas
	}

	// Apply patch
	switch v := obj.(type) {
	case *appsv1.Deployment:
		v.Spec.Replicas = &newReplicas
	case *appsv1.StatefulSet:
		v.Spec.Replicas = &newReplicas
	}

	return nil
}
