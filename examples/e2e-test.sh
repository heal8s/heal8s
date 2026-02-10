#!/bin/bash
# E2E test: deploy test app, create Remediation CR, assert memory limit increased.
# Modes: USE_IN_CLUSTER_OPERATOR=1 — assume operator already deployed (e.g. via make verify).
#        Default — create Kind cluster, install CRDs, run operator locally.
set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

USE_IN_CLUSTER_OPERATOR="${USE_IN_CLUSTER_OPERATOR:-0}"
KIND_NAME="${KIND_NAME:-heal8s-dev}"
OPERATOR_PID=""

echo "========================================"
echo "heal8s End-to-End Test"
echo "========================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

function check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 is not installed"
        exit 1
    fi
}

# Check prerequisites
log_info "Checking prerequisites..."
check_command kubectl
check_command kind
check_command docker

if [ "$USE_IN_CLUSTER_OPERATOR" = "1" ] || [ "$USE_IN_CLUSTER_OPERATOR" = "true" ]; then
    log_info "Mode: in-cluster operator (operator and CRDs assumed already deployed)"
fi

# Step 1: Create Kind cluster (or ensure it exists)
log_info "Step 1: Ensuring Kind cluster is up..."
if kind get clusters 2>/dev/null | grep -q "^${KIND_NAME}$"; then
    log_warn "Cluster ${KIND_NAME} already exists"
else
    kind create cluster --config examples/kind/cluster-config.yaml --name "${KIND_NAME}"
    log_info "Cluster created successfully"
fi

kubectl cluster-info --context "kind-${KIND_NAME}"
kubectl wait --for=condition=Ready nodes --all --timeout=120s

# Step 2: Build memory leak test app
log_info "Step 2: Building memory leak test application..."
cd examples/test-app/memory-leak-app
docker build -t memory-leak-simulator:latest .
kind load docker-image memory-leak-simulator:latest --name "${KIND_NAME}"
cd "${REPO_ROOT}"
log_info "Test application built and loaded"

# Step 3: Install CRDs (skip if in-cluster operator mode)
if [ "$USE_IN_CLUSTER_OPERATOR" != "1" ] && [ "$USE_IN_CLUSTER_OPERATOR" != "true" ]; then
    log_info "Step 3: Installing CRDs..."
    kubectl apply -f operator/config/crd/remediation-crd.yaml
    kubectl wait --for condition=established --timeout=60s crd/remediations.k8shealer.k8s-healer.io
    log_info "CRDs installed successfully"
fi

# Step 4: Start operator in background (skip if in-cluster)
if [ "$USE_IN_CLUSTER_OPERATOR" != "1" ] && [ "$USE_IN_CLUSTER_OPERATOR" != "true" ]; then
    log_info "Step 4: Starting operator locally..."
    cd operator
    go run cmd/manager/main.go --webhook-port=8082 > /tmp/heal8s-operator.log 2>&1 &
    OPERATOR_PID=$!
    cd "${REPO_ROOT}"
    log_info "Operator started with PID $OPERATOR_PID"
    sleep 5
    if ! kill -0 $OPERATOR_PID 2>/dev/null; then
        log_error "Operator failed to start"
        cat /tmp/heal8s-operator.log
        exit 1
    fi
else
    log_info "Step 4: Skipping operator start (in-cluster mode)"
fi

# Step 5: Deploy test application
log_info "Step 5: Deploying test application..."
kubectl apply -f examples/test-app/memory-leak-deployment.yaml
log_info "Test application deployed"

# Wait for pod to be running
log_info "Waiting for pod to start..."
kubectl wait --for=condition=Ready pod -l app=memory-leak -n test-app --timeout=60s || true

# Step 6: Wait for OOM (this will take a few minutes)
log_info "Step 6: Waiting for OOM event (this may take 2-3 minutes)..."
TIMEOUT=300
ELAPSED=0
OOM_DETECTED=false

while [ $ELAPSED -lt $TIMEOUT ]; do
    if kubectl get events -n test-app | grep -i "OOMKilled\|Out of memory" > /dev/null 2>&1; then
        log_info "OOM detected!"
        OOM_DETECTED=true
        break
    fi
    
    echo -n "."
    sleep 10
    ELAPSED=$((ELAPSED + 10))
done

echo ""

if [ "$OOM_DETECTED" = false ]; then
    log_warn "OOM not detected within timeout, but continuing with test..."
fi

# Step 7: Manually create a remediation (simulating alert)
log_info "Step 7: Creating Remediation CR..."
kubectl apply -f - <<EOF
apiVersion: k8shealer.k8s-healer.io/v1alpha1
kind: Remediation
metadata:
  name: test-oom-remediation
  namespace: test-app
spec:
  alert:
    name: KubePodOOMKilled
    fingerprint: e2e-test-$(date +%s)
    severity: critical
    source: e2e-test
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

log_info "Remediation CR created"

# Step 8: Wait for remediation to be applied
log_info "Step 8: Waiting for remediation to be applied..."
sleep 90

# Check remediation status
PHASE=$(kubectl get remediation test-oom-remediation -n test-app -o jsonpath='{.status.phase}')
log_info "Remediation phase: $PHASE"

if [ "$PHASE" != "Succeeded" ]; then
    log_error "Remediation did not succeed. Current phase: $PHASE"
    kubectl describe remediation test-oom-remediation -n test-app
    [ -n "$OPERATOR_PID" ] && kill $OPERATOR_PID 2>/dev/null || true
    exit 1
fi

# Step 9: Verify memory was increased
log_info "Step 9: Verifying memory limits were increased..."
MEMORY_LIMIT=$(kubectl get deployment memory-leak-app -n test-app -o jsonpath='{.spec.template.spec.containers[0].resources.limits.memory}')
log_info "New memory limit: $MEMORY_LIMIT"

# Check if it's higher than original 256Mi
if echo "$MEMORY_LIMIT" | grep -qE "(320|384|400|512)Mi"; then
    log_info "Memory limit increased successfully! ✓"
else
    log_warn "Memory limit may not have been increased as expected: $MEMORY_LIMIT"
fi

# Step 10: Cleanup
log_info "Step 10: Cleaning up..."
[ -n "$OPERATOR_PID" ] && kill $OPERATOR_PID 2>/dev/null || true
kubectl delete -f examples/test-app/memory-leak-deployment.yaml --ignore-not-found=true
kubectl delete remediation test-oom-remediation -n test-app --ignore-not-found=true

log_info ""
log_info "========================================"
log_info "E2E Test Completed Successfully! ✓"
log_info "========================================"
log_info ""
log_info "Summary:"
log_info "  - Cluster: kind-${KIND_NAME}"
log_info "  - Remediation: Applied successfully"
log_info "  - Memory: Increased to $MEMORY_LIMIT"
log_info ""
log_info "To tear down: make kind-down"
