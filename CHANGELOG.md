# Changelog

All notable changes to heal8s will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-10

### Added

#### Core Features
- Kubernetes Operator for self-healing automation
- GitHub App service for GitOps PR workflow
- Remediation Custom Resource Definition (CRD)
- Alert routing and processing system
- Alertmanager webhook integration

#### Remediation Scenarios
- **IncreaseMemory**: Automatic memory limit increases for OOM events
- **ScaleUp**: Replica scaling when HPA maxes out
- **RollbackImage**: Image rollback for crash loops

#### Observability
- Prometheus metrics endpoint
- Custom metrics for alerts, remediations, and durations
- ServiceMonitor for Prometheus Operator
- PrometheusRule for heal8s health alerts
- Kubernetes resource alert examples

#### Deployment
- Production-ready Helm chart
- RBAC configuration (ClusterRole, ClusterRoleBinding)
- ServiceAccount setup
- ConfigMap for alert routing
- Security contexts and pod security standards

#### Automation
- GitHub Actions workflow for automatic PR application
- Multi-environment support (dev/staging/prod)
- Automated deployment verification
- Remediation CR status updates

#### Testing & CI
- Unit tests for remediation logic, alert routing, YAML patching, controller
- End-to-end test script and memory-leak simulator test app
- Full verify pipeline: `make verify` (Kind, Helm, Remediation, memory-limit assert)
- Pre-commit hooks (`make test-unit`, `make lint`)
- GitHub Actions: test/lint on PR; verify on push to main/master
- Helm chart RBAC for leader election (`coordination.k8s.io/leases`) and workload update (deployments/statefulsets/daemonsets)

#### Documentation
- Comprehensive README with quick start
- Architecture documentation with diagrams
- Helm chart deployment guide
- GitHub Actions setup guide
- Contributing guidelines
- Implementation summary

#### Developer Experience
- Go workspace for monorepo
- Root Makefile for common tasks
- Docker Compose for local services
- Kind cluster configuration
- Quick start script
- Structured logging with zap

### Technical Details

#### Operator
- Built with controller-runtime v0.17.0
- Go 1.22
- CRD API version: k8shealer.k8s-healer.io/v1alpha1
- Alert deduplication with TTL
- Async alert processing
- Leader election support

#### GitHub App
- Out-of-cluster Kubernetes client
- GitHub App authentication
- Strategic YAML patching
- Polling-based CR watching
- Configurable via YAML or environment

#### Monitoring
- 7 custom Prometheus metrics
- Alert rules for system health
- Kubernetes resource alerts
- Histogram for duration tracking

### Dependencies

#### Operator
- kubernetes (client-go, api, apimachinery): v0.29.0
- controller-runtime: v0.17.0
- prometheus/client_golang: v1.23.2
- prometheus/alertmanager: v0.27.0

#### GitHub App
- go-github/v57: v57.0.0
- ghinstallation/v2: v2.9.0
- client-go: v0.29.0
- controller-runtime: v0.17.0

### Repository Structure
```
heal8s/
├── operator/               # Kubernetes Operator
├── github-app/            # GitHub App Service
├── charts/heal8s/         # Helm chart
├── examples/              # Examples and test apps
├── docs/                  # Documentation
└── Makefile              # Root automation
```

### Security
- Apache 2.0 License
- RBAC with least-privilege
- No hardcoded credentials
- Service account-based auth
- Secure kubeconfig management
- Pod security contexts

### Known Limitations
- Single-cluster support only
- GitHub App must run out-of-cluster initially
- PostgreSQL integration not yet implemented
- No web dashboard UI

### Breaking Changes
None - initial release

---

## [Unreleased]

### Added
- **Web dashboard**: Operator serves a simple web UI on port 8082 (same as webhook). Shows in-memory event stream: alerts received, remediations applied/failed, with target, action and details (e.g. memory limit 256Mi → 384Mi). Routes: `/`, `/dashboard`, `/api/events` (JSON). Handy for local debugging: `kubectl port-forward -n heal8s-system deployment/heal8s-operator 8082:8082` then open http://localhost:8082/

### Planned
- PostgreSQL audit trail
- Multi-cluster support
- Slack/Discord notifications
- AI-powered suggestions
- Additional remediation scenarios
- Cost optimization features

[0.1.0]: https://github.com/heal8s/heal8s/releases/tag/v0.1.0
[Unreleased]: https://github.com/heal8s/heal8s/compare/v0.1.0...HEAD
