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

	k8shealerv1alpha1 "github.com/heal8s/heal8s/operator/api/v1alpha1"
)

// Alert represents a simplified Alertmanager alert
type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Status      string            `json:"status"`
	StartsAt    string            `json:"startsAt"`
	Fingerprint string            `json:"fingerprint"`
}

// RouterConfig holds the configuration for alert routing
type RouterConfig struct {
	Routes map[string]RouteConfig
}

// RouteConfig defines how to handle a specific alert type
type RouteConfig struct {
	ActionType ActionType
	Params     map[string]string
}

// ActionType represents the remediation action type
type ActionType string

const (
	ActionTypeIncreaseMemory ActionType = "IncreaseMemory"
	ActionTypeScaleUp        ActionType = "ScaleUp"
	ActionTypeRollbackImage  ActionType = "RollbackImage"
)

// DefaultRouterConfig returns the default alert routing configuration
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		Routes: map[string]RouteConfig{
			"KubePodOOMKilled": {
				ActionType: ActionTypeIncreaseMemory,
				Params: map[string]string{
					"memoryIncreasePercent": "25",
					"maxMemory":             "2Gi",
				},
			},
			"ContainerOOMKilled": {
				ActionType: ActionTypeIncreaseMemory,
				Params: map[string]string{
					"memoryIncreasePercent": "25",
					"maxMemory":             "2Gi",
				},
			},
			"KubeHpaMaxedOut": {
				ActionType: ActionTypeScaleUp,
				Params: map[string]string{
					"scaleUpPercent": "50",
					"maxReplicas":    "10",
				},
			},
			"KubePodCrashLooping": {
				ActionType: ActionTypeRollbackImage,
				Params: map[string]string{
					"rollbackMaxRevisions": "5",
				},
			},
		},
	}
}

// RouteAlert determines the remediation action for an alert
func RouteAlert(alert Alert, config RouterConfig) (*k8shealerv1alpha1.RemediationSpec, error) {
	alertname := alert.Labels["alertname"]
	if alertname == "" {
		return nil, fmt.Errorf("alert has no alertname label")
	}

	route, ok := config.Routes[alertname]
	if !ok {
		return nil, fmt.Errorf("no route configured for alert: %s", alertname)
	}

	// Extract target information from alert labels
	target, err := extractTargetFromAlert(alert)
	if err != nil {
		return nil, fmt.Errorf("failed to extract target from alert: %w", err)
	}

	spec := &k8shealerv1alpha1.RemediationSpec{
		Alert: k8shealerv1alpha1.AlertInfo{
			Name:        alertname,
			Fingerprint: alert.Fingerprint,
			Source:      "alertmanager",
			Severity:    alert.Labels["severity"],
		},
		Target: *target,
		Action: k8shealerv1alpha1.Action{
			Type:   k8shealerv1alpha1.ActionType(route.ActionType),
			Params: route.Params,
		},
		Strategy: k8shealerv1alpha1.Strategy{
			Mode:            k8shealerv1alpha1.StrategyModeGitOps,
			RequireApproval: true,
			Environment:     alert.Labels["environment"],
			TTL:             "24h",
		},
	}

	return spec, nil
}

// extractTargetFromAlert extracts target resource information from alert labels
func extractTargetFromAlert(alert Alert) (*k8shealerv1alpha1.TargetResource, error) {
	namespace := alert.Labels["namespace"]
	if namespace == "" {
		return nil, fmt.Errorf("alert has no namespace label")
	}

	// Try to determine the resource kind and name from labels
	// Common patterns:
	// - pod: "pod_name" -> extract deployment/statefulset from pod owner
	// - deployment: "deployment"
	// - statefulset: "statefulset"

	var kind, name, container string

	// Check for deployment
	if deploymentName := alert.Labels["deployment"]; deploymentName != "" {
		kind = "Deployment"
		name = deploymentName
	} else if statefulsetName := alert.Labels["statefulset"]; statefulsetName != "" {
		kind = "StatefulSet"
		name = statefulsetName
	} else if podName := alert.Labels["pod"]; podName != "" {
		// Try to infer from pod name (e.g., "api-service-5f7b8c9d-xyz" -> "api-service")
		// This is a simplified approach - in production, you'd query the pod and get owner ref
		kind = "Deployment"
		// Extract base name (remove replica set hash and pod hash)
		// This is approximate - better to query the actual pod
		name = extractDeploymentNameFromPod(podName)
	} else {
		return nil, fmt.Errorf("cannot determine target resource from alert labels")
	}

	// Get container name if available
	container = alert.Labels["container"]

	return &k8shealerv1alpha1.TargetResource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Container: container,
	}, nil
}

// extractDeploymentNameFromPod tries to extract deployment name from pod name
// Pod naming pattern: <deployment>-<replicaset-hash>-<pod-hash>
func extractDeploymentNameFromPod(podName string) string {
	// Simple heuristic: remove last two segments (replicaset hash and pod hash)
	// In production, you should query the pod and get owner references

	// For now, just return the pod name - the controller will need to handle this
	return podName
}
