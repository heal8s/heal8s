package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RemediationPhase represents the current phase of the remediation process
// +kubebuilder:validation:Enum=Pending;Analyzing;PRCreated;Applying;Succeeded;Failed;Expired
type RemediationPhase string

const (
	RemediationPhasePending   RemediationPhase = "Pending"
	RemediationPhaseAnalyzing RemediationPhase = "Analyzing"
	RemediationPhasePRCreated RemediationPhase = "PRCreated"
	RemediationPhaseApplying  RemediationPhase = "Applying"
	RemediationPhaseSucceeded RemediationPhase = "Succeeded"
	RemediationPhaseFailed    RemediationPhase = "Failed"
	RemediationPhaseExpired   RemediationPhase = "Expired"
)

// ActionType represents the type of remediation action to take
// +kubebuilder:validation:Enum=IncreaseMemory;ScaleUp;RollbackImage;CustomScript
type ActionType string

const (
	ActionTypeIncreaseMemory ActionType = "IncreaseMemory"
	ActionTypeScaleUp        ActionType = "ScaleUp"
	ActionTypeRollbackImage  ActionType = "RollbackImage"
	ActionTypeCustomScript   ActionType = "CustomScript"
)

// StrategyMode represents the remediation strategy
// +kubebuilder:validation:Enum=GitOps;Direct
type StrategyMode string

const (
	StrategyModeGitOps StrategyMode = "GitOps"
	StrategyModeDirect StrategyMode = "Direct"
)

// RemediationSpec defines the desired state of Remediation
type RemediationSpec struct {
	// Alert information from Alertmanager
	Alert AlertInfo `json:"alert"`

	// Target resource to remediate
	Target TargetResource `json:"target"`

	// Action to take for remediation
	Action Action `json:"action"`

	// Strategy for applying the remediation
	Strategy Strategy `json:"strategy"`

	// GitHub integration settings
	// +optional
	GitHub *GitHubConfig `json:"github,omitempty"`
}

// AlertInfo contains information about the alert that triggered the remediation
type AlertInfo struct {
	// Name of the alert (e.g., "KubePodOOMKilled")
	Name string `json:"name"`

	// AlertID is a unique identifier for this alert instance
	// +optional
	AlertID string `json:"alertId,omitempty"`

	// Fingerprint is the Alertmanager fingerprint for deduplication
	Fingerprint string `json:"fingerprint"`

	// Source of the alert (e.g., "alertmanager")
	Source string `json:"source"`

	// Severity of the alert
	// +kubebuilder:validation:Enum=critical;warning;info
	Severity string `json:"severity"`

	// Payload is the raw alert payload (JSON)
	// +optional
	Payload string `json:"payload,omitempty"`
}

// TargetResource identifies the Kubernetes resource to remediate
type TargetResource struct {
	// Kind of the resource (Deployment, StatefulSet, DaemonSet)
	// +kubebuilder:validation:Enum=Deployment;StatefulSet;DaemonSet
	Kind string `json:"kind"`

	// Name of the resource
	Name string `json:"name"`

	// Namespace of the resource
	Namespace string `json:"namespace"`

	// Container name (if multiple containers in pod)
	// +optional
	Container string `json:"container,omitempty"`
}

// Action defines what remediation action to take
type Action struct {
	// Type of action
	Type ActionType `json:"type"`

	// Parameters for the action (varies by type)
	// +optional
	Params map[string]string `json:"params,omitempty"`
}

// Strategy defines how the remediation should be applied
type Strategy struct {
	// Mode: GitOps or Direct
	// +kubebuilder:default=GitOps
	Mode StrategyMode `json:"mode"`

	// RequireApproval indicates if human approval is needed
	// +kubebuilder:default=true
	RequireApproval bool `json:"requireApproval"`

	// Environment (e.g., "prod", "staging", "dev")
	// +optional
	Environment string `json:"environment,omitempty"`

	// TTL for the remediation (e.g., "24h")
	// +kubebuilder:default="24h"
	// +optional
	TTL string `json:"ttl,omitempty"`
}

// GitHubConfig contains GitHub integration settings
type GitHubConfig struct {
	// Enabled indicates if GitHub integration is active
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Owner is the GitHub repository owner
	Owner string `json:"owner"`

	// Repo is the GitHub repository name
	Repo string `json:"repo"`

	// BaseBranch is the base branch for PRs
	// +kubebuilder:default=main
	BaseBranch string `json:"baseBranch"`

	// ManifestPath is the path template to the manifest file
	// Supports interpolation: {environment}, {namespace}, {name}
	ManifestPath string `json:"manifestPath"`

	// PRTitleTemplate is the PR title template
	// +optional
	PRTitleTemplate string `json:"prTitleTemplate,omitempty"`

	// PRLabels are labels to add to the PR
	// +optional
	PRLabels []string `json:"prLabels,omitempty"`

	// AutoMerge indicates if the PR should be auto-merged
	// +kubebuilder:default=false
	AutoMerge bool `json:"autoMerge"`
}

// RemediationStatus defines the observed state of Remediation
type RemediationStatus struct {
	// Phase is the current phase of the remediation
	Phase RemediationPhase `json:"phase,omitempty"`

	// Reason provides a human-readable explanation of the phase
	// +optional
	Reason string `json:"reason,omitempty"`

	// PRNumber is the GitHub PR number
	// +optional
	PRNumber int `json:"prNumber,omitempty"`

	// PRURL is the full URL to the GitHub PR
	// +optional
	PRURL string `json:"prUrl,omitempty"`

	// CommitSHA is the Git commit SHA
	// +optional
	CommitSHA string `json:"commitSHA,omitempty"`

	// AppliedAt is when the remediation was applied
	// +optional
	AppliedAt *metav1.Time `json:"appliedAt,omitempty"`

	// ResolvedAt is when the remediation was resolved
	// +optional
	ResolvedAt *metav1.Time `json:"resolvedAt,omitempty"`

	// Attempts is the number of remediation attempts
	// +optional
	Attempts int `json:"attempts,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Remediation is the Schema for the remediations API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=rem
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target.name`
// +kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action.type`
// +kubebuilder:printcolumn:name="PR",type=integer,JSONPath=`.status.prNumber`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Remediation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemediationSpec   `json:"spec,omitempty"`
	Status RemediationStatus `json:"status,omitempty"`
}

// RemediationList contains a list of Remediation
// +kubebuilder:object:root=true
type RemediationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Remediation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Remediation{}, &RemediationList{})
}
