package k8s

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8shealerv1alpha1 "github.com/heal8s/heal8s/github-app/pkg/api/v1alpha1"
)

// Client wraps a Kubernetes client
type Client struct {
	client.Client
	scheme *runtime.Scheme
}

// NewClient creates a new Kubernetes client from kubeconfig
func NewClient(kubeconfig string) (*Client, error) {
	scheme := runtime.NewScheme()
	if err := k8shealerv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add scheme: %w", err)
	}

	var config *rest.Config
	var err error

	if kubeconfig != "" {
		// Out-of-cluster: use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	} else {
		// In-cluster: use in-cluster config
		config, err = ctrl.GetConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	}

	cl, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{
		Client: cl,
		scheme: scheme,
	}, nil
}

// ListPendingRemediations lists all Remediation CRs that are pending GitHub PR creation
func (c *Client) ListPendingRemediations(ctx context.Context, namespace string) (*k8shealerv1alpha1.RemediationList, error) {
	list := &k8shealerv1alpha1.RemediationList{}

	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := c.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("failed to list remediations: %w", err)
	}

	// Filter for pending with GitHub enabled
	pending := &k8shealerv1alpha1.RemediationList{}
	for _, item := range list.Items {
		if item.Status.Phase == k8shealerv1alpha1.RemediationPhasePending &&
			item.Spec.GitHub != nil &&
			item.Spec.GitHub.Enabled {
			pending.Items = append(pending.Items, item)
		}
	}

	return pending, nil
}

// UpdateRemediationStatus updates the status of a Remediation CR
func (c *Client) UpdateRemediationStatus(ctx context.Context, remediation *k8shealerv1alpha1.Remediation) error {
	if err := c.Status().Update(ctx, remediation); err != nil {
		return fmt.Errorf("failed to update remediation status: %w", err)
	}
	return nil
}
