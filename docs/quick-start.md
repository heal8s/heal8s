# heal8s Quick Start Guide

This guide will help you get heal8s up and running in a local Kind cluster for development and testing.

## Option A: Run everything in Docker (isolated, nothing on host)

Only **Docker** is required; no Go, Helm or Kind on the host.

1. **Clone and enter the repo:**
   ```bash
   git clone https://github.com/heal8s/heal8s.git
   cd heal8s
   ```

2. **Configure Docker (optional):** Copy `.env.example` to `.env`. In `.env` you can set:
   - **DOCKER_HOST** — leave `unix:///var/run/docker.sock` for local Docker, or `tcp://host:2375` for a remote daemon.
   - **DOCKER_REGISTRY**, **DOCKER_REGISTRY_USER**, **DOCKER_REGISTRY_TOKEN** — only if you need to log into a registry (e.g. private base images or push). Put the token in `DOCKER_REGISTRY_TOKEN`; `.env` is gitignored.

3. **Run the full pipeline:**
   ```bash
   make all-in-docker
   ```
   This builds the CI image and runs tests, lint, Kind, Helm deploy and verify inside it.

See [README — Fully isolated](../README.md#fully-isolated-recommended-if-you-want-nothing-on-the-host) for the table of all `.env` variables.

---

## Option B: Local tools (Go, Helm, Kind on host)

## Prerequisites

- Go 1.22+ (operator module requires Go 1.23)
- Docker
- kubectl, Helm 3
- Kind (Kubernetes in Docker)
- (Optional) A GitHub account for GitOps/PR flow

## Step 1: Clone the Repository

```bash
git clone https://github.com/heal8s/heal8s.git
cd heal8s
```

## Step 2: Create Local Kind Cluster

```bash
make kind-up
```

This creates a Kind cluster named `heal8s-dev` with port mappings for Alertmanager webhooks.

Verify the cluster:

```bash
kubectl cluster-info --context kind-heal8s-dev
kubectl get nodes
```

## One-command verify (alternative to Steps 3–4)

To run the full pipeline (Kind, Helm deploy, test app, Remediation, assert) in one go:

```bash
make verify
```

This installs CRDs via the Helm chart, deploys the operator, creates a test Remediation, and checks that memory limits were updated. Skip to Step 7 for manual testing, or use the steps below for a manual flow.

## Step 3: Install CRDs and operator

**Option A — Deploy with Helm (recommended):**

```bash
make docker-operator
make kind-load-operator
make deploy-operator-helm
```

This installs the Remediation CRD from the chart and deploys the operator in `heal8s-system`.

**Option B — CRDs only, then run operator locally:**

```bash
make install-operator
```

Then in one terminal:

```bash
make run-operator
```

The operator will start the Alertmanager webhook on port 8082 and watch for Remediation CRs.

Verify CRDs:

```bash
kubectl get crds | grep remediation
```

## Step 4: (Optional) Run the Operator Locally

Only if you chose Option B in Step 3:

```bash
make run-operator
```

You should see logs like:

```
INFO starting manager
INFO starting Alertmanager webhook server address=:8082
```

## Step 5: Configure GitHub App (Optional for Testing)

For full GitOps workflow, you need to create a GitHub App. For now, we'll test with Direct mode which doesn't require GitHub integration.

### Creating a GitHub App (Full Setup)

1. Go to GitHub Settings → Developer settings → GitHub Apps → New GitHub App

2. Configure:
   - **Name**: `heal8s-dev-yourname`
   - **Homepage URL**: `https://github.com/heal8s/heal8s`
   - **Webhook**: Uncheck "Active"
   - **Repository permissions**:
     - Contents: Read & Write
     - Pull requests: Read & Write
   - **Where can this GitHub App be installed?**: Only on this account

3. Create the app and note the **App ID**

4. Generate a private key and download it

5. Install the app on a repository containing your Kubernetes manifests

6. Note the **Installation ID** (visible in the URL when managing the installation)

## Step 6: Run GitHub App Service (Optional)

Create a config file `github-app/config/config.yaml`:

```yaml
github:
  appID: YOUR_APP_ID
  installationID: YOUR_INSTALLATION_ID
  privateKeyPath: /path/to/private-key.pem

kubernetes:
  kubeconfig: ~/.kube/config
  namespace: ""

processor:
  pollInterval: 10s
  batchSize: 10
```

In another terminal window:

```bash
cd github-app
go run cmd/server/main.go --config config/config.yaml
```

## Step 7: Deploy Test Application

Deploy an application that will trigger OOM:

```bash
kubectl apply -f examples/test-app/oom-deployment.yaml
```

This creates a test application with low memory limits that will OOM.

Watch the pods:

```bash
kubectl get pods -n test-app -w
```

You should see the pod getting OOMKilled.

## Step 8: Simulate Alert (Manual Testing)

Since we don't have Prometheus/Alertmanager running yet, we can manually create a Remediation CR:

```bash
kubectl apply -f - <<EOF
apiVersion: k8shealer.k8s-healer.io/v1alpha1
kind: Remediation
metadata:
  name: test-oom-remediation
  namespace: test-app
spec:
  alert:
    name: KubePodOOMKilled
    fingerprint: test-123
    severity: critical
    source: manual
  target:
    kind: Deployment
    name: oom-test-app
    namespace: test-app
    container: app
  action:
    type: IncreaseMemory
    params:
      memoryIncreasePercent: "50"
      maxMemory: "512Mi"
  strategy:
    mode: Direct
    requireApproval: false
    ttl: 24h
EOF
```

## Step 9: Watch the Remediation

Watch the Remediation CR:

```bash
kubectl get remediations -n test-app -w
```

Check the status:

```bash
kubectl describe remediation test-oom-remediation -n test-app
```

For Direct mode, the operator will immediately patch the Deployment:

```bash
kubectl get deployment oom-test-app -n test-app -o yaml | grep -A 5 resources
```

You should see the memory limits increased!

## Step 10: Verify the Fix

Check that the pod is no longer OOMKilling:

```bash
kubectl get pods -n test-app
```

## Testing GitOps Mode

To test the full GitOps workflow with GitHub PRs:

1. Create a GitHub repository with your Kubernetes manifests
2. Configure the GitHub App (Step 5)
3. Run the GitHub App service (Step 6)
4. Create a Remediation with `strategy.mode: GitOps`:

```yaml
apiVersion: k8shealer.k8s-healer.io/v1alpha1
kind: Remediation
metadata:
  name: test-gitops-remediation
  namespace: test-app
spec:
  alert:
    name: KubePodOOMKilled
    fingerprint: test-456
    severity: critical
    source: manual
  target:
    kind: Deployment
    name: oom-test-app
    namespace: test-app
    container: app
  action:
    type: IncreaseMemory
    params:
      memoryIncreasePercent: "50"
      maxMemory: "512Mi"
  strategy:
    mode: GitOps
    requireApproval: true
    environment: dev
    ttl: 24h
  github:
    enabled: true
    owner: YOUR_GITHUB_USERNAME
    repo: YOUR_MANIFESTS_REPO
    baseBranch: main
    manifestPath: "k8s/test-app/oom-test-app.yaml"
    prTitleTemplate: "[heal8s] {action}: {target}"
    prLabels:
      - heal8s
      - auto-remediation
    autoMerge: false
```

5. The GitHub App will create a PR in your repository
6. Review and merge the PR
7. (Optional) Set up GitHub Actions to apply the changes

## Next Steps

1. **Set up Prometheus and Alertmanager**: See [Alertmanager Setup Guide](alertmanager-setup.md)
2. **Configure alert routing**: Customize alert rules in `operator/config/manager/config.yaml`
3. **Deploy to production**: Use Helm chart for production deployment
4. **Set up monitoring**: Configure Prometheus metrics and Grafana dashboards

## Troubleshooting

### Operator not starting

Check logs:

```bash
kubectl logs -n heal8s-system deployment/heal8s-operator
```

### GitHub App not creating PRs

Check GitHub App service logs and verify:
- GitHub App credentials are correct
- Repository permissions are granted
- Manifest path exists in repository

### Remediation stuck in Pending

Check operator logs for errors in reconciliation:

```bash
kubectl logs -n heal8s-system deployment/heal8s-operator | grep remediation
```

## Cleanup

To tear down the development environment:

```bash
# Delete test resources
kubectl delete -f examples/test-app/oom-deployment.yaml

# Delete Kind cluster
make kind-down

# Stop operator and GitHub App (Ctrl+C in terminals)
```

## Common Commands

```bash
# List all remediations
kubectl get remediations --all-namespaces

# Describe a remediation
kubectl describe remediation <name> -n <namespace>

# Watch remediations
kubectl get remediations -A -w

# Delete a remediation
kubectl delete remediation <name> -n <namespace>

# View operator logs
kubectl logs -f -n heal8s-system deployment/heal8s-operator

# Port-forward to operator webhook (for testing)
kubectl port-forward -n heal8s-system deployment/heal8s-operator 8082:8082
```

## Testing Alert Reception

To test the Alertmanager webhook integration:

```bash
# Send a test alert to the operator
curl -X POST http://localhost:8082/webhooks/alertmanager \
  -H "Content-Type: application/json" \
  -d '{
    "version": "4",
    "receiver": "heal8s",
    "status": "firing",
    "alerts": [
      {
        "status": "firing",
        "labels": {
          "alertname": "KubePodOOMKilled",
          "namespace": "test-app",
          "pod": "oom-test-app-abc123",
          "deployment": "oom-test-app",
          "container": "app",
          "severity": "critical"
        },
        "annotations": {
          "description": "Pod oom-test-app-abc123 was OOMKilled"
        },
        "startsAt": "2025-02-10T12:00:00Z",
        "fingerprint": "test-123"
      }
    ]
  }'
```

Check if a Remediation CR was created:

```bash
kubectl get remediations -n test-app
```
