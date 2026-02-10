#!/usr/bin/env bash
# Run inside CI container. Repo is mounted at /workspace.
# Usage: ci-run.sh [test-unit | lint | verify]
# For verify: run container with -v /var/run/docker.sock:/var/run/docker.sock
set -e

cd /workspace

run_test_unit() {
  echo "[ci] Running unit tests (operator + github-app)..."
  (cd operator && go test ./...)
  (cd github-app && go test ./...)
}

run_lint() {
  echo "[ci] Linting operator..."
  (cd operator && go fmt ./... && go vet ./...)
  echo "[ci] Linting github-app..."
  (cd github-app && go fmt ./... && go vet ./...)
  echo "[ci] Linting Helm chart..."
  helm lint charts/heal8s
}

run_verify() {
  # Run unit tests and lint first (same order as make verify)
  run_test_unit
  run_lint

  if ! docker info >/dev/null 2>&1; then
    echo "[ci] ERROR: Docker socket not available. Run with: -v /var/run/docker.sock:/var/run/docker.sock"
    exit 1
  fi
  export IMG="${IMG:-heal8s/operator:dev}"
  # Always use a dedicated cluster and config in CI (avoids port conflict with host cluster)
  export KIND_NAME="heal8s-ci"
  export KIND_CLUSTER_CONFIG="/workspace/examples/kind/cluster-config-ci.yaml"
  export KUBECONFIG="${KUBECONFIG:-/workspace/.kube/ci-config}"
  mkdir -p "$(dirname "$KUBECONFIG")"
  # Remove existing CI cluster so we get a fresh context in our KUBECONFIG
  if kind get clusters 2>/dev/null | grep -q "^${KIND_NAME}$"; then
    kind delete cluster --name "${KIND_NAME}" 2>/dev/null || true
  fi

  # Registry login from .env (DOCKER_REGISTRY_TOKEN etc.) if set
  if [ -n "${DOCKER_REGISTRY_TOKEN:-}" ] && [ -n "${DOCKER_REGISTRY:-}" ]; then
    echo "[ci] Logging into registry ${DOCKER_REGISTRY}..."
    echo "${DOCKER_REGISTRY_TOKEN}" | docker login -u "${DOCKER_REGISTRY_USER:-token}" --password-stdin "${DOCKER_REGISTRY}" || true
  fi

  echo "[ci] Building operator image..."
  docker build -t "${IMG}" -f operator/Dockerfile operator/
  echo "[ci] Running E2E verify pipeline (Kind + Helm + Remediation + assert)..."
  ./scripts/verify.sh
}

case "${1:-verify}" in
  test-unit) run_test_unit ;;
  lint)      run_lint ;;
  verify)    run_verify ;;
  *)
    echo "Usage: $0 {test-unit|lint|verify}"
    exit 1
    ;;
esac
