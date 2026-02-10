# AGENTS.md — heal8s

## 1. Purpose

This repository is **heal8s**: an open-source hybrid self-healing system for Kubernetes.

The role of AI agents (Cursor, Claude, Copilot, etc.) is to **help write and refactor code**, not to make product decisions. The project owner is human. Any contentious decisions, architecture changes, and API changes must be confirmed by them.

---

## 2. High-level architecture

The project consists of:

- `operator/` — Kubernetes Operator (Go, controller-runtime), CRD `Remediation`, Alertmanager webhook.
- `github-app/` — external GitHub App service (Go) that:
  - watches `Remediation` objects via the K8s API,
  - creates PRs in GitHub with updated manifests,
  - will send notifications to Slack (later).
- `charts/` — Helm charts for deploying the operator (and later github-app).
- `examples/` — Alertmanager config, GitHub Actions, kind/k3d setup examples.
- `docs/` — architecture and usage documentation.

When changing architecture, update **documentation** first (spec-driven approach) [web:161][web:155].

---

## 3. Development workflow (spec-driven + vibe coding)

### 3.1. Spec-driven layer (required)

- Before major changes (new remediation scenario, new CRD, API changes) **update README/docs first**:
  - describe the flow,
  - list inputs/outputs,
  - document limitations and edge cases explicitly.
- Spec attitude:
  - The spec is the **source of truth** for system behavior [web:161][web:172].
  - Code must follow the spec, not the other way around.
- If the agent sees a mismatch between code and documentation:
  - do not fix it silently,
  - propose a diff for docs and call it out in a separate comment/commit message.
- **After each new implementation** (feature, scenario, UI, API): update **CHANGELOG.md** ([Unreleased] — Added/Changed/Fixed) and, if needed, **README.md**, **docs/** and other markdown files so documentation stays in sync with code.

### 3.2. Vibe coding layer (allowed)

- Vibe coding is allowed for:
  - boilerplate (CRD types, controllers, YAML),
  - GitHub Actions, Helm charts,
  - quick remediator logic prototypes.
- Important:
  - changes must be **small and meaningful**,
  - each step should be documented in the PR with what was done.

---

## 4. Codebase semantics & context

AI agents may use **semantic search and code graph understanding** [web:154][web:160]:

- Keep functions short with clear names; avoid "god objects".
- If the task requires understanding relationships (who calls the controller, how the CRD is used):
  - build a mental picture first (dependency list, who talks to whom),
  - then make changes.

If it is unclear how a part of the code works — **ask the user/repo owner** instead of guessing.

---

## 5. Languages & style

- Primary language for code and comments: **English**.
- Commits: in English, informative (`feat: add OOMKill remediation controller`).
- Discussions may be in any language, but everything that lands in code/docs must be in English.

Go code:
- Follow Go idioms (error first, small functions, context.Context).
- Use structured logging (zap/logr); do not use `fmt.Println` in production.
- Do not add unnecessary abstractions.

---

## 6. Safety & permissions

AI agents **must NOT**:

- Delete files, change permissions, or touch `.git` without explicit request.
- Change security settings (RBAC, NetworkPolicy, Secrets) without confirmation.
- Add external dependencies without justification (why this library).

**Ask explicitly** before:

- Mass refactoring across the repo.
- CRD API changes (breaking changes).
- Changing operator behavior in `prod` by default.
- Any changes affecting cluster security (RBAC expansion, Secret access).

---

## 7. Build & test

Commands are run from the repo root (`make` uses the root Makefile).

### Isolated run (everything in containers, only Docker on host)

- **All in Docker**: `make all-in-docker` — builds the CI image (`Dockerfile.ci`); inside: tests, lint, operator image build, Kind, Helm, verify. No Go, Helm, Kind, kubectl needed on the host.
- Docker connection and registry token: copy `.env.example` to `.env`; set `DOCKER_HOST` if needed (default local: `unix:///var/run/docker.sock` or `tcp://...` for remote); for registry login use `DOCKER_REGISTRY`, `DOCKER_REGISTRY_USER`, `DOCKER_REGISTRY_TOKEN`. See README § "Fully isolated" and docs/quick-start.md § "Option A".
- Separately: `make test-unit-in-docker`, `make lint-in-docker`, `make verify-in-docker`. For verify, Docker access is required (socket or DOCKER_HOST from .env).

### Local run (Go, Helm, Kind on host)

- **Unit tests**: `make test-unit` (or `make test`) — tests in `operator/` and `github-app/`, no cluster.
- **Lint**: `make lint` — `go fmt` and `go vet` in both modules, plus `helm lint charts/heal8s`.
- **Full verify (E2E in one command)**: `make verify` — unit tests → lint → build operator Docker image → Kind up → load image → deploy operator via Helm (CRDs from chart) → deploy test app (memory-leak) → create Remediation CR in `heal8s-system` → wait ~90s → assert phase=Succeeded and memory limit increase. Optionally keep cluster: `LEAVE_CLUSTER=1 make verify`.
- **E2E only** (cluster already exists or created by script): `make e2e` or `./examples/e2e-test.sh`. "Operator in cluster" mode (operator already deployed via `make verify`): `USE_IN_CLUSTER_OPERATOR=1 ./examples/e2e-test.sh`.
- **Pre-commit**: install hooks — `pre-commit install`. Manual run — `pre-commit run --all-files`. Hooks run `make test-unit` and `make lint`.

CI (GitHub Actions) on push/PR: job "Test and Lint" — `make test-unit`, `make lint`; on push to `main`/`master` additionally job "Verify" — `make verify` (Kind + deploy + limit checks).

```bash
# Isolated (only Docker on host)
make all-in-docker         # tests + lint + verify in container
make test-unit-in-docker
make lint-in-docker
make verify-in-docker

# Local (Go, Helm, Kind on host)
make test-unit
make lint
make verify          # full cycle with Kind and E2E
make e2e             # E2E only (local operator or USE_IN_CLUSTER_OPERATOR=1)
make docker-operator # build operator image (IMG=heal8s/operator:dev)
make kind-up
make kind-load-operator
make deploy-operator-helm  # CRDs + Helm (image already loaded in Kind)

# Operator (from operator/)
cd operator
make build
make test
make run

# GitHub App (from github-app/)
cd github-app
go test ./...
go run ./cmd/server

# Helm chart
helm lint charts/heal8s

# Reload operator in Kind after rebuilding image
kind load docker-image heal8s/operator:dev --name heal8s-dev
kubectl rollout restart deployment/heal8s-operator -n heal8s-system --context kind-heal8s-dev

# Port-forward to operator dashboard (events and remediations)
kubectl port-forward -n heal8s-system deployment/heal8s-operator 8082:8082
# Open in browser: http://localhost:8082/
```

**Cursor Terminal Allow List:** add to Cursor settings (Settings → Terminal → Command Allowlist) the commands the agent/terminal may run without confirmation: `make test-unit`, `make lint`, `make verify`, `make all-in-docker`, `make docker-operator`, `kind load docker-image`, `kubectl rollout restart`, `kubectl port-forward`.
