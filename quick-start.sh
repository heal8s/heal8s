#!/bin/bash
# heal8s Quick Start Script

set -e

echo "🚀 heal8s Quick Start"
echo "===================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

function step() {
    echo -e "${BLUE}▶${NC} $1"
}

function success() {
    echo -e "${GREEN}✓${NC} $1"
}

function info() {
    echo -e "${YELLOW}ℹ${NC} $1"
}

# Step 1: Create Kind cluster
step "Creating Kind cluster..."
if kind get clusters | grep -q "heal8s-dev"; then
    info "Cluster already exists"
else
    make kind-up
    success "Cluster created"
fi

# Step 2: Build memory leak app
step "Building memory leak test application..."
cd examples/test-app/memory-leak-app
docker build -t memory-leak-simulator:latest .
kind load docker-image memory-leak-simulator:latest --name heal8s-dev
cd ../../..
success "Test app built and loaded"

# Step 3: Install CRDs
step "Installing CRDs..."
make install-operator
success "CRDs installed"

# Step 4: Start operator (in background)
step "Starting operator..."
cd operator
go run cmd/manager/main.go > /tmp/heal8s-operator.log 2>&1 &
OPERATOR_PID=$!
echo $OPERATOR_PID > /tmp/heal8s-operator.pid
cd ..
sleep 5
success "Operator started (PID: $OPERATOR_PID)"

# Step 5: Deploy test application
step "Deploying memory leak test app..."
kubectl apply -f examples/test-app/memory-leak-deployment.yaml
kubectl wait --for=condition=Ready pod -l app=memory-leak -n test-app --timeout=60s || true
success "Test app deployed"

echo ""
echo "✅ heal8s is now running!"
echo ""
echo "📊 Monitor the system:"
echo "   kubectl get remediations -A -w"
echo "   kubectl top pods -n test-app"
echo "   kubectl get events -n test-app --watch"
echo ""
echo "🔬 Check operator logs:"
echo "   tail -f /tmp/heal8s-operator.log"
echo ""
echo "🧪 Run E2E test:"
echo "   ./examples/e2e-test.sh"
echo ""
echo "🛑 Stop operator:"
echo "   kill $(cat /tmp/heal8s-operator.pid)"
echo ""
echo "🗑️  Cleanup:"
echo "   make kind-down"
