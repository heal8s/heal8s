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

package remediate

import (
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyIncreaseMemory increases the memory limits and requests for a container
func ApplyIncreaseMemory(obj client.Object, containerName string, params map[string]string) error {
	// Parse parameters
	increasePercent := 25 // default
	if p, ok := params["memoryIncreasePercent"]; ok {
		if val, err := strconv.Atoi(p); err == nil {
			increasePercent = val
		}
	}

	maxMemory := resource.MustParse("2Gi") // default
	if m, ok := params["maxMemory"]; ok {
		if parsed, err := resource.ParseQuantity(m); err == nil {
			maxMemory = parsed
		}
	}

	// Get containers based on object type
	var containers []corev1.Container
	switch v := obj.(type) {
	case *appsv1.Deployment:
		containers = v.Spec.Template.Spec.Containers
	case *appsv1.StatefulSet:
		containers = v.Spec.Template.Spec.Containers
	case *appsv1.DaemonSet:
		containers = v.Spec.Template.Spec.Containers
	default:
		return fmt.Errorf("unsupported object type: %T", obj)
	}

	// Find the target container
	containerIndex := -1
	if containerName == "" && len(containers) == 1 {
		containerIndex = 0
	} else {
		for i, c := range containers {
			if c.Name == containerName {
				containerIndex = i
				break
			}
		}
	}

	if containerIndex == -1 {
		return fmt.Errorf("container %s not found", containerName)
	}

	container := &containers[containerIndex]

	// Get current memory limit
	currentMemory := container.Resources.Limits[corev1.ResourceMemory]
	if currentMemory.IsZero() {
		// No limit set, use a default starting point
		currentMemory = resource.MustParse("256Mi")
	}

	// Calculate new memory (increase by percentage)
	multiplier := 1.0 + float64(increasePercent)/100.0
	newMemoryValue := int64(float64(currentMemory.Value()) * multiplier)

	// Round to nearest 64Mi for cleaner values
	roundTo := int64(64 * 1024 * 1024) // 64Mi in bytes
	newMemoryValue = ((newMemoryValue + roundTo - 1) / roundTo) * roundTo

	newMemory := *resource.NewQuantity(newMemoryValue, resource.BinarySI)

	// Cap at maxMemory
	if newMemory.Cmp(maxMemory) > 0 {
		newMemory = maxMemory
	}

	// Update container resources
	if container.Resources.Limits == nil {
		container.Resources.Limits = corev1.ResourceList{}
	}
	if container.Resources.Requests == nil {
		container.Resources.Requests = corev1.ResourceList{}
	}

	container.Resources.Limits[corev1.ResourceMemory] = newMemory
	// Set requests equal to limits for predictability
	container.Resources.Requests[corev1.ResourceMemory] = newMemory

	// Update the container in the object
	switch v := obj.(type) {
	case *appsv1.Deployment:
		v.Spec.Template.Spec.Containers[containerIndex] = *container
	case *appsv1.StatefulSet:
		v.Spec.Template.Spec.Containers[containerIndex] = *container
	case *appsv1.DaemonSet:
		v.Spec.Template.Spec.Containers[containerIndex] = *container
	}

	return nil
}

// CalculateMemoryIncrease calculates the new memory value without applying it
func CalculateMemoryIncrease(current resource.Quantity, percent int, max resource.Quantity) resource.Quantity {
	multiplier := 1.0 + float64(percent)/100.0
	newMemoryValue := int64(float64(current.Value()) * multiplier)

	// Round to nearest 64Mi
	roundTo := int64(64 * 1024 * 1024)
	newMemoryValue = ((newMemoryValue + roundTo - 1) / roundTo) * roundTo

	newMemory := *resource.NewQuantity(newMemoryValue, resource.BinarySI)

	if newMemory.Cmp(max) > 0 {
		return max
	}

	return newMemory
}
