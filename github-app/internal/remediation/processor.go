package remediation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v57/github"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	githubclient "github.com/heal8s/heal8s/github-app/internal/github"
	"github.com/heal8s/heal8s/github-app/internal/k8s"
	"github.com/heal8s/heal8s/github-app/internal/yaml"
	k8shealerv1alpha1 "github.com/heal8s/heal8s/github-app/pkg/api/v1alpha1"
)

// Processor processes Remediation CRs and creates GitHub PRs
type Processor struct {
	k8sClient    *k8s.Client
	githubClient *githubclient.Client
	yamlPatcher  *yaml.Patcher
	logger       logr.Logger
	namespace    string
}

// NewProcessor creates a new remediation processor
func NewProcessor(k8sClient *k8s.Client, githubClient *githubclient.Client, logger logr.Logger, namespace string) *Processor {
	return &Processor{
		k8sClient:    k8sClient,
		githubClient: githubClient,
		yamlPatcher:  yaml.NewPatcher(),
		logger:       logger,
		namespace:    namespace,
	}
}

// Run starts the processor loop
func (p *Processor) Run(ctx context.Context, pollInterval time.Duration) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	p.logger.Info("starting remediation processor", "pollInterval", pollInterval)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("processor stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := p.processPendingRemediations(ctx); err != nil {
				p.logger.Error(err, "failed to process pending remediations")
			}
		}
	}
}

func (p *Processor) processPendingRemediations(ctx context.Context) error {
	// List all pending remediations
	remediations, err := p.k8sClient.ListPendingRemediations(ctx, p.namespace)
	if err != nil {
		return fmt.Errorf("failed to list pending remediations: %w", err)
	}

	if len(remediations.Items) == 0 {
		return nil
	}

	p.logger.Info("found pending remediations", "count", len(remediations.Items))

	// Process each remediation
	for _, rem := range remediations.Items {
		if err := p.processRemediation(ctx, &rem); err != nil {
			p.logger.Error(err, "failed to process remediation",
				"name", rem.Name,
				"namespace", rem.Namespace)
			// Continue processing others
		}
	}

	return nil
}

func (p *Processor) processRemediation(ctx context.Context, remediation *k8shealerv1alpha1.Remediation) error {
	logger := p.logger.WithValues("remediation", remediation.Name)
	logger.Info("processing remediation")

	// Validate GitHub config
	if remediation.Spec.GitHub == nil {
		return fmt.Errorf("GitHub config is nil")
	}

	ghConfig := remediation.Spec.GitHub

	// Interpolate manifest path
	manifestPath := p.interpolateManifestPath(ghConfig.ManifestPath, remediation)

	// Fetch manifest from GitHub
	logger.Info("fetching manifest from GitHub", "path", manifestPath)
	manifest, err := p.fetchManifestFromGitHub(ctx, ghConfig.Owner, ghConfig.Repo, manifestPath, ghConfig.BaseBranch)
	if err != nil {
		return p.updateStatusFailed(ctx, remediation, fmt.Sprintf("Failed to fetch manifest: %v", err))
	}

	// Patch manifest
	logger.Info("patching manifest")
	patchedManifest, err := p.yamlPatcher.PatchManifest(manifest, remediation)
	if err != nil {
		return p.updateStatusFailed(ctx, remediation, fmt.Sprintf("Failed to patch manifest: %v", err))
	}

	// Create PR
	logger.Info("creating GitHub PR")
	prTitle := p.generatePRTitle(remediation, ghConfig.PRTitleTemplate)
	prBody := p.generatePRBody(remediation)
	headBranch := p.generateBranchName(remediation)

	prReq := &githubclient.PRRequest{
		Owner:         ghConfig.Owner,
		Repo:          ghConfig.Repo,
		Title:         prTitle,
		Body:          prBody,
		BaseBranch:    ghConfig.BaseBranch,
		HeadBranch:    headBranch,
		FilePath:      manifestPath,
		FileContent:   patchedManifest,
		CommitMessage: fmt.Sprintf("heal8s: %s for %s/%s", remediation.Spec.Action.Type, remediation.Spec.Target.Namespace, remediation.Spec.Target.Name),
	}

	prNumber, prURL, err := p.githubClient.CreatePR(ctx, prReq)
	if err != nil {
		return p.updateStatusFailed(ctx, remediation, fmt.Sprintf("Failed to create PR: %v", err))
	}

	logger.Info("PR created successfully", "number", prNumber, "url", prURL)

	// Update remediation status
	return p.updateStatusPRCreated(ctx, remediation, prNumber, prURL)
}

func (p *Processor) fetchManifestFromGitHub(ctx context.Context, owner, repo, path, branch string) (string, error) {
	fileContent, _, _, err := p.githubClient.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	})
	if err != nil {
		return "", err
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", err
	}

	return content, nil
}

func (p *Processor) interpolateManifestPath(template string, remediation *k8shealerv1alpha1.Remediation) string {
	result := template
	result = strings.ReplaceAll(result, "{environment}", remediation.Spec.Strategy.Environment)
	result = strings.ReplaceAll(result, "{namespace}", remediation.Spec.Target.Namespace)
	result = strings.ReplaceAll(result, "{name}", remediation.Spec.Target.Name)
	return result
}

func (p *Processor) generatePRTitle(remediation *k8shealerv1alpha1.Remediation, template string) string {
	if template == "" {
		template = "[heal8s] {action}: {target} in {namespace}"
	}

	result := template
	result = strings.ReplaceAll(result, "{action}", string(remediation.Spec.Action.Type))
	result = strings.ReplaceAll(result, "{target}", remediation.Spec.Target.Name)
	result = strings.ReplaceAll(result, "{namespace}", remediation.Spec.Target.Namespace)
	result = strings.ReplaceAll(result, "{alert}", remediation.Spec.Alert.Name)

	return result
}

func (p *Processor) generatePRBody(remediation *k8shealerv1alpha1.Remediation) string {
	body := fmt.Sprintf(`## heal8s Automatic Remediation

**Alert**: %s
**Severity**: %s
**Target**: %s/%s (%s)
**Action**: %s

### Details

This PR was automatically generated in response to a %s alert in %s.

**Fingerprint**: %s
**Timestamp**: %s

### Changes

`,
		remediation.Spec.Alert.Name,
		remediation.Spec.Alert.Severity,
		remediation.Spec.Target.Namespace,
		remediation.Spec.Target.Name,
		remediation.Spec.Target.Kind,
		remediation.Spec.Action.Type,
		remediation.Spec.Alert.Name,
		remediation.Spec.Target.Namespace,
		remediation.Spec.Alert.Fingerprint,
		remediation.CreationTimestamp.Format(time.RFC3339),
	)

	switch remediation.Spec.Action.Type {
	case k8shealerv1alpha1.ActionTypeIncreaseMemory:
		body += fmt.Sprintf("- Increase memory limits by %s%%\n", remediation.Spec.Action.Params["memoryIncreasePercent"])
		body += fmt.Sprintf("- Maximum memory: %s\n", remediation.Spec.Action.Params["maxMemory"])
	case k8shealerv1alpha1.ActionTypeScaleUp:
		body += fmt.Sprintf("- Scale up by %s%%\n", remediation.Spec.Action.Params["scaleUpPercent"])
		body += fmt.Sprintf("- Maximum replicas: %s\n", remediation.Spec.Action.Params["maxReplicas"])
	}

	body += "\n### Review Checklist\n\n"
	body += "- [ ] Verify the changes are appropriate\n"
	body += "- [ ] Check resource limits and quotas\n"
	body += "- [ ] Ensure no sensitive data is exposed\n"
	body += "- [ ] Merge when ready\n"

	return body
}

func (p *Processor) generateBranchName(remediation *k8shealerv1alpha1.Remediation) string {
	timestamp := remediation.CreationTimestamp.Format("20060102-150405")
	return fmt.Sprintf("heal8s/%s/%s-%s",
		strings.ToLower(string(remediation.Spec.Action.Type)),
		remediation.Spec.Target.Name,
		timestamp)
}

func (p *Processor) updateStatusPRCreated(ctx context.Context, remediation *k8shealerv1alpha1.Remediation, prNumber int, prURL string) error {
	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhasePRCreated
	remediation.Status.Reason = "GitHub PR created successfully"
	remediation.Status.PRNumber = prNumber
	remediation.Status.PRURL = prURL
	now := metav1.Now()
	remediation.Status.LastUpdateTime = &now

	return p.k8sClient.UpdateRemediationStatus(ctx, remediation)
}

func (p *Processor) updateStatusFailed(ctx context.Context, remediation *k8shealerv1alpha1.Remediation, reason string) error {
	remediation.Status.Phase = k8shealerv1alpha1.RemediationPhaseFailed
	remediation.Status.Reason = reason
	now := metav1.Now()
	remediation.Status.LastUpdateTime = &now
	remediation.Status.ResolvedAt = &now

	return p.k8sClient.UpdateRemediationStatus(ctx, remediation)
}
