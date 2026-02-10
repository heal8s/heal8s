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

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	k8shealerv1alpha1 "github.com/heal8s/heal8s/operator/api/v1alpha1"
	"github.com/heal8s/heal8s/operator/internal/remediate"
)

// RemediationReconciler reconciles a Remediation object
type RemediationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8shealer.k8s-healer.io,resources=remediations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8shealer.k8s-healer.io,resources=remediations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8shealer.k8s-healer.io,resources=remediations/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;daemonsets,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RemediationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Remediation instance
	remediation := &k8shealerv1alpha1.Remediation{}
	if err := r.Get(ctx, req.NamespacedName, remediation); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Remediation resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Remediation")
		return ctrl.Result{}, err
	}

	// Handle different phases
	switch remediation.Status.Phase {
	case "": // New remediation
		return r.handleNewRemediation(ctx, remediation)
	case k8shealerv1alpha1.RemediationPhasePending:
		return r.handlePendingRemediation(ctx, remediation)
	case k8shealerv1alpha1.RemediationPhaseAnalyzing:
		return r.handleAnalyzingRemediation(ctx, remediation)
	case k8shealerv1alpha1.RemediationPhaseApplying:
		return r.handleApplyingRemediation(ctx, remediation)
	case k8shealerv1alpha1.RemediationPhaseSucceeded,
		k8shealerv1alpha1.RemediationPhaseFailed,
		k8shealerv1alpha1.RemediationPhaseExpired:
		// Terminal states - nothing to do
		return ctrl.Result{}, nil
	case k8shealerv1alpha1.RemediationPhasePRCreated:
		// Waiting for external service and human approval
		return r.handlePRCreated(ctx, remediation)
	default:
		logger.Info("Unknown phase", "phase", remediation.Status.Phase)
		return ctrl.Result{}, nil
	}
}

func (r *RemediationReconciler) handleNewRemediation(ctx context.Context, remediation *k8shealerv1alpha1.Remediation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling new remediation")

	// Update status to Pending
	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhasePending
	remediation.Status.Reason = "Remediation created, waiting for processing"
	now := metav1.Now()
	remediation.Status.LastUpdateTime = &now

	// Add condition
	meta.SetStatusCondition(&remediation.Status.Conditions, metav1.Condition{
		Type:               "AlertReceived",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: remediation.Generation,
		LastTransitionTime: now,
		Reason:             "AlertReceived",
		Message:            fmt.Sprintf("Alert %s received from %s", remediation.Spec.Alert.Name, remediation.Spec.Alert.Source),
	})

	if err := r.Status().Update(ctx, remediation); err != nil {
		logger.Error(err, "Failed to update Remediation status to Pending")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *RemediationReconciler) handlePendingRemediation(ctx context.Context, remediation *k8shealerv1alpha1.Remediation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling pending remediation")

	// Validate target resource exists
	targetKey := client.ObjectKey{
		Namespace: remediation.Spec.Target.Namespace,
		Name:      remediation.Spec.Target.Name,
	}

	var targetObj client.Object
	switch remediation.Spec.Target.Kind {
	case "Deployment":
		targetObj = &appsv1.Deployment{}
	case "StatefulSet":
		targetObj = &appsv1.StatefulSet{}
	case "DaemonSet":
		targetObj = &appsv1.DaemonSet{}
	default:
		return r.updateStatusToFailed(ctx, remediation, fmt.Sprintf("Unsupported target kind: %s", remediation.Spec.Target.Kind))
	}

	if err := r.Get(ctx, targetKey, targetObj); err != nil {
		if apierrors.IsNotFound(err) {
			return r.updateStatusToFailed(ctx, remediation, fmt.Sprintf("Target resource not found: %s/%s", remediation.Spec.Target.Namespace, remediation.Spec.Target.Name))
		}
		logger.Error(err, "Failed to get target resource")
		return ctrl.Result{}, err
	}

	// Move to Analyzing phase
	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhaseAnalyzing
	remediation.Status.Reason = "Analyzing target resource and calculating remediation"
	now := metav1.Now()
	remediation.Status.LastUpdateTime = &now

	if err := r.Status().Update(ctx, remediation); err != nil {
		logger.Error(err, "Failed to update status to Analyzing")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *RemediationReconciler) handleAnalyzingRemediation(ctx context.Context, remediation *k8shealerv1alpha1.Remediation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling analyzing remediation")

	// Check remediation strategy
	if remediation.Spec.Strategy.Mode == k8shealerv1alpha1.StrategyModeDirect && !remediation.Spec.Strategy.RequireApproval {
		// Direct mode without approval - apply immediately
		return r.handleDirectRemediation(ctx, remediation)
	}

	// GitOps mode - wait for external service
	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhasePending
	remediation.Status.Reason = "Waiting for GitHub App service to create PR"
	now := metav1.Now()
	remediation.Status.LastUpdateTime = &now

	if err := r.Status().Update(ctx, remediation); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// External service will watch this and create PR
	return ctrl.Result{}, nil
}

func (r *RemediationReconciler) handleDirectRemediation(ctx context.Context, remediation *k8shealerv1alpha1.Remediation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Applying direct remediation")

	// Get target resource
	targetKey := client.ObjectKey{
		Namespace: remediation.Spec.Target.Namespace,
		Name:      remediation.Spec.Target.Name,
	}

	var targetObj client.Object
	switch remediation.Spec.Target.Kind {
	case "Deployment":
		targetObj = &appsv1.Deployment{}
	default:
		return r.updateStatusToFailed(ctx, remediation, fmt.Sprintf("Direct remediation not implemented for kind: %s", remediation.Spec.Target.Kind))
	}

	if err := r.Get(ctx, targetKey, targetObj); err != nil {
		return r.updateStatusToFailed(ctx, remediation, fmt.Sprintf("Failed to get target: %v", err))
	}

	// Apply remediation based on action type
	var err error
	switch remediation.Spec.Action.Type {
	case k8shealerv1alpha1.ActionTypeIncreaseMemory:
		err = remediate.ApplyIncreaseMemory(targetObj, remediation.Spec.Target.Container, remediation.Spec.Action.Params)
	case k8shealerv1alpha1.ActionTypeScaleUp:
		err = remediate.ApplyScaleUp(targetObj, remediation.Spec.Action.Params)
	case k8shealerv1alpha1.ActionTypeRollbackImage:
		err = remediate.ApplyRollbackImage(ctx, r.Client, targetObj, remediation.Spec.Action.Params)
	default:
		return r.updateStatusToFailed(ctx, remediation, fmt.Sprintf("Unsupported action type: %s", remediation.Spec.Action.Type))
	}

	if err != nil {
		return r.updateStatusToFailed(ctx, remediation, fmt.Sprintf("Failed to calculate remediation: %v", err))
	}

	// Update status to Applying
	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhaseApplying
	remediation.Status.Reason = "Applying remediation directly to cluster"
	now := metav1.Now()
	remediation.Status.LastUpdateTime = &now
	remediation.Status.Attempts++

	if err := r.Status().Update(ctx, remediation); err != nil {
		logger.Error(err, "Failed to update status to Applying")
		return ctrl.Result{}, err
	}

	// Apply the patch
	if err := r.Update(ctx, targetObj); err != nil {
		logger.Error(err, "Failed to update target resource")
		return r.updateStatusToFailed(ctx, remediation, fmt.Sprintf("Failed to apply remediation: %v", err))
	}

	// Update to Succeeded
	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhaseSucceeded
	remediation.Status.Reason = "Remediation applied successfully"
	applied := metav1.Now()
	remediation.Status.AppliedAt = &applied
	remediation.Status.ResolvedAt = &applied
	remediation.Status.LastUpdateTime = &applied

	meta.SetStatusCondition(&remediation.Status.Conditions, metav1.Condition{
		Type:               "Applied",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: remediation.Generation,
		LastTransitionTime: applied,
		Reason:             "RemediationApplied",
		Message:            "Remediation applied directly to cluster",
	})

	if err := r.Status().Update(ctx, remediation); err != nil {
		logger.Error(err, "Failed to update status to Succeeded")
		return ctrl.Result{}, err
	}

	logger.Info("Remediation applied successfully")
	return ctrl.Result{}, nil
}

func (r *RemediationReconciler) handleApplyingRemediation(ctx context.Context, remediation *k8shealerv1alpha1.Remediation) (ctrl.Result, error) {
	// This phase is used when GitHub Actions applies the remediation
	// For now, we just wait for external update
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *RemediationReconciler) handlePRCreated(ctx context.Context, remediation *k8shealerv1alpha1.Remediation) (ctrl.Result, error) {
	// PR is created, waiting for merge and application
	// Check TTL
	if remediation.Spec.Strategy.TTL != "" {
		ttl, err := time.ParseDuration(remediation.Spec.Strategy.TTL)
		if err == nil {
			if time.Since(remediation.CreationTimestamp.Time) > ttl {
				return r.updateStatusToExpired(ctx, remediation, "Remediation TTL exceeded")
			}
		}
	}

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *RemediationReconciler) updateStatusToFailed(ctx context.Context, remediation *k8shealerv1alpha1.Remediation, reason string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Updating remediation status to Failed", "reason", reason)

	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhaseFailed
	remediation.Status.Reason = reason
	now := metav1.Now()
	remediation.Status.LastUpdateTime = &now
	remediation.Status.ResolvedAt = &now

	meta.SetStatusCondition(&remediation.Status.Conditions, metav1.Condition{
		Type:               "Failed",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: remediation.Generation,
		LastTransitionTime: now,
		Reason:             "RemediationFailed",
		Message:            reason,
	})

	if err := r.Status().Update(ctx, remediation); err != nil {
		logger.Error(err, "Failed to update status to Failed")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RemediationReconciler) updateStatusToExpired(ctx context.Context, remediation *k8shealerv1alpha1.Remediation, reason string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Updating remediation status to Expired", "reason", reason)

	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhaseExpired
	remediation.Status.Reason = reason
	now := metav1.Now()
	remediation.Status.LastUpdateTime = &now
	remediation.Status.ResolvedAt = &now

	if err := r.Status().Update(ctx, remediation); err != nil {
		logger.Error(err, "Failed to update status to Expired")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemediationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8shealerv1alpha1.Remediation{}).
		Complete(r)
}
