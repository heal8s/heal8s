#!/bin/bash
set -e

echo "Creating Kind cluster for heal8s development..."
kind create cluster --config=cluster-config.yaml

echo ""
echo "Cluster created successfully!"
echo ""
echo "To use this cluster, run:"
echo "  kubectl cluster-info --context kind-heal8s-dev"
echo ""
echo "Next steps:"
echo "  1. Install CRDs: cd ../.. && make install-operator"
echo "  2. Run operator: make run-operator"
echo "  3. Deploy test app: kubectl apply -f ../test-app/oom-deployment.yaml"
