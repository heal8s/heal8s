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
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyRollbackImage rolls back a workload to the previous stable image
func ApplyRollbackImage(ctx context.Context, cl client.Client, obj client.Object, params map[string]string) error {
	maxRevisions := 5 // default
	if m, ok := params["rollbackMaxRevisions"]; ok {
		if val, err := strconv.Atoi(m); err == nil {
			maxRevisions = val
		}
	}

	switch v := obj.(type) {
	case *appsv1.Deployment:
		return rollbackDeployment(ctx, cl, v, maxRevisions)
	default:
		return fmt.Errorf("rollback not supported for type: %T", obj)
	}
}

func rollbackDeployment(ctx context.Context, cl client.Client, deployment *appsv1.Deployment, maxRevisions int) error {
	// Get the annotation for the last stable image
	lastStableImage := deployment.Annotations["heal8s.io/last-stable-image"]

	if lastStableImage == "" {
		// Try to get previous revision from ReplicaSets
		// This is a simplified approach - in production you'd query ReplicaSets
		// and find the previous stable version
		return fmt.Errorf("no previous stable image found (annotation heal8s.io/last-stable-image missing)")
	}

	// Find the container and update its image
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]
		// Store current image as "attempted" image
		if deployment.Annotations == nil {
			deployment.Annotations = make(map[string]string)
		}
		deployment.Annotations["heal8s.io/attempted-image"] = container.Image

		// Rollback to last stable
		container.Image = lastStableImage
	}

	return nil
}

// MarkImageAsStable marks the current image as stable (to be called after successful deployment)
func MarkImageAsStable(deployment *appsv1.Deployment) {
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}

	// Store the first container's image as stable
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		currentImage := deployment.Spec.Template.Spec.Containers[0].Image
		deployment.Annotations["heal8s.io/last-stable-image"] = currentImage
	}
}
