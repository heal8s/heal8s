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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyScaleUp increases the number of replicas for a workload
func ApplyScaleUp(obj client.Object, params map[string]string) error {
	// Parse parameters
	scalePercent := 50 // default
	if p, ok := params["scaleUpPercent"]; ok {
		if val, err := strconv.Atoi(p); err == nil {
			scalePercent = val
		}
	}

	maxReplicas := int32(10) // default
	if m, ok := params["maxReplicas"]; ok {
		if val, err := strconv.Atoi(m); err == nil {
			maxReplicas = int32(val)
		}
	}

	// Get current replicas based on object type
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

	// Cap at maxReplicas
	if newReplicas > maxReplicas {
		newReplicas = maxReplicas
	}

	// Update replicas
	switch v := obj.(type) {
	case *appsv1.Deployment:
		v.Spec.Replicas = &newReplicas
	case *appsv1.StatefulSet:
		v.Spec.Replicas = &newReplicas
	}

	return nil
}

// CalculateScaleUp calculates the new replica count without applying it
func CalculateScaleUp(current int32, percent int, max int32) int32 {
	increase := int32(float64(current) * float64(percent) / 100.0)
	if increase < 1 {
		increase = 1
	}

	newReplicas := current + increase

	if newReplicas > max {
		return max
	}

	return newReplicas
}
