#!/usr/bin/env bash
# Full verification pipeline: Kind, deploy operator (Helm), test app, Remediation CR, assert memory limit.
# Run from repo root. Assumes: unit tests and lint already passed, operator image already built (use make verify).
set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

IMG="${IMG:-heal8s/operator:dev}"
KIND_NAME="${KIND_NAME:-heal8s-dev}"
KIND_CLUSTER_CONFIG="${KIND_CLUSTER_CONFIG:-examples/kind/cluster-config.yaml}"
LEAVE_CLUSTER="${LEAVE_CLUSTER:-0}"
WAIT_REMEDIATION="${WAIT_REMEDIATION:-90}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[verify]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[verify]${NC} $*"; }
log_error() { echo -e "${RED}[verify]${NC} $*"; }

check_cmd() {
  for c in "$@"; do
    if ! command -v "$c" &>/dev/null; then
      log_error "Required command not found: $c"
      exit 1
    fi
  done
}

log_info "Prerequisites check..."
check_cmd kubectl kind docker helm

# 1. Kind up
log_info "Step 1: Ensure Kind cluster is up..."
if kind get clusters 2>/dev/null | grep -q "^${KIND_NAME}$"; then
  log_warn "Cluster ${KIND_NAME} already exists"
else
  kind create cluster --config "${KIND_CLUSTER_CONFIG}" --name "${KIND_NAME}"
fi
kubectl cluster-info --context "kind-${KIND_NAME}"
kubectl wait --for=condition=Ready nodes --all --timeout=120s

# 2. Load operator image
log_info "Step 2: Load operator image into Kind..."
kind load docker-image "${IMG}" --name "${KIND_NAME}"

# 3. Deploy operator via Helm (includes CRDs from chart)
log_info "Step 3: Deploy operator via Helm..."
helm upgrade --install heal8s charts/heal8s \
  --namespace heal8s-system \
  --create-namespace \
  --set operator.image.repository="${IMG%:*}" \
  --set operator.image.tag="${IMG##*:}" \
  --set operator.image.pullPolicy=IfNotPresent \
  --wait --timeout=120s

kubectl wait --for=condition=established --timeout=60s crd/remediations.k8shealer.k8s-healer.io 2>/dev/null || true
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=heal8s -n heal8s-system --timeout=120s

# 4. Build and load test app image, deploy test app
log_info "Step 4: Build and deploy memory-leak test app..."
docker build -t memory-leak-simulator:latest examples/test-app/memory-leak-app
kind load docker-image memory-leak-simulator:latest --name "${KIND_NAME}"

kubectl apply -f examples/test-app/memory-leak-deployment.yaml
kubectl wait --for=condition=Ready pod -l app=memory-leak -n test-app --timeout=120s || true

# 5. Create Remediation CR (in operator namespace so controller sees it; target stays in test-app)
REMEDIATION_NS="heal8s-system"
kubectl delete remediation verify-oom-remediation -n "${REMEDIATION_NS}" --ignore-not-found=true || true
log_info "Step 5: Create Remediation CR (IncreaseMemory) in ${REMEDIATION_NS}..."
kubectl apply -f - <<EOF
apiVersion: k8shealer.k8s-healer.io/v1alpha1
kind: Remediation
metadata:
  name: verify-oom-remediation
  namespace: ${REMEDIATION_NS}
spec:
  alert:
    name: KubePodOOMKilled
    fingerprint: verify-$(date +%s)
    severity: critical
    source: verify-script
  target:
    kind: Deployment
    name: memory-leak-app
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
    ttl: 1h
EOF

# 6. Wait for reconciliation
log_info "Step 6: Waiting ${WAIT_REMEDIATION}s for reconciliation..."
sleep "$WAIT_REMEDIATION"

# 7. Assert remediation phase and memory limit
log_info "Step 7: Assert remediation succeeded and memory limit increased..."
PHASE=$(kubectl get remediation verify-oom-remediation -n "${REMEDIATION_NS}" -o jsonpath='{.status.phase}')
MEMORY_LIMIT=$(kubectl get deployment memory-leak-app -n test-app -o jsonpath='{.spec.template.spec.containers[0].resources.limits.memory}')

if [ "$PHASE" != "Succeeded" ]; then
  log_error "Remediation phase is '$PHASE', expected 'Succeeded'"
  kubectl describe remediation verify-oom-remediation -n "${REMEDIATION_NS}"
  log_info "Operator logs (last 80 lines):"
  kubectl logs -n heal8s-system -l app.kubernetes.io/name=heal8s --tail=80 || true
  exit 1
fi

# Original limit 256Mi, +50% => 384Mi (or 320Mi for different rounding)
if ! echo "$MEMORY_LIMIT" | grep -qE "(320|384|400|512)Mi"; then
  log_error "Memory limit '$MEMORY_LIMIT' was not increased as expected (expected 320Mi-512Mi)"
  exit 1
fi

log_info "Remediation phase: $PHASE, memory limit: $MEMORY_LIMIT — OK"

# 8. Optional cleanup
if [ "$LEAVE_CLUSTER" = "1" ] || [ "$LEAVE_CLUSTER" = "true" ]; then
  log_warn "Leaving cluster and test resources (LEAVE_CLUSTER=1)"
else
  log_info "Step 8: Cleanup test resources (deployment and remediation)..."
  kubectl delete -f examples/test-app/memory-leak-deployment.yaml --ignore-not-found=true || true
  kubectl delete remediation verify-oom-remediation -n "${REMEDIATION_NS}" --ignore-not-found=true || true
  log_info "Tear down cluster with: make kind-down"
fi

log_info "Verify completed successfully."
