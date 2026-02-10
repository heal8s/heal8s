# Contributing to heal8s

Thank you for your interest in contributing to heal8s! This document provides guidelines and information for contributors.

## Code of Conduct

This project adheres to a code of conduct. By participating, you are expected to uphold this code. Please be respectful and constructive in all interactions.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/heal8s.git`
3. Create a feature branch: `git checkout -b feature/my-feature`
4. Make your changes
5. Test your changes
6. Commit with clear messages
7. Push to your fork
8. Open a Pull Request

## Development Setup

See the [Quick Start Guide](docs/quick-start.md) for setting up a local development environment.

### Prerequisites

- Go 1.22+
- Docker
- kubectl
- Kind

### Building

```bash
# Build operator
cd operator && go build -o bin/manager cmd/manager/main.go

# Build GitHub App
cd github-app && go build -o bin/github-app cmd/server/main.go
```

### Running Tests

```bash
# Unit tests (from repo root)
make test-unit

# Lint (fmt, vet, helm lint)
make lint

# Full verify (Kind + Helm + E2E assertion)
make verify
```

See [AGENTS.md](AGENTS.md) §7 and [Quick Start](docs/quick-start.md) for details.

## Project Structure

```
heal8s/
├── operator/              # Kubernetes operator
│   ├── api/v1alpha1/     # CRD definitions
│   ├── internal/         # Internal packages
│   └── cmd/manager/      # Main entrypoint
├── github-app/           # GitHub App service
│   ├── cmd/server/       # Main entrypoint
│   └── internal/         # Internal packages
├── examples/             # Example configurations
├── docs/                 # Documentation
└── charts/               # Helm charts
```

## Coding Guidelines

### Go Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `go vet` before committing
- Add comments for exported functions
- Write table-driven tests

### Commit Messages

Use conventional commits format:

```
type(scope): brief description

More detailed explanation if needed.

Fixes #123
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

**Examples**:
```
feat(operator): add support for StatefulSet remediation
fix(github-app): handle manifest not found error
docs: update quick start guide
```

## Adding New Remediation Types

To add a new remediation action (e.g., RollbackImage):

1. **Update CRD**: Add action type to enum in `operator/api/v1alpha1/remediation_types.go`

2. **Add Router Logic**: Update `operator/internal/remediate/router.go` to map alerts to new action

3. **Implement Logic**: Create `operator/internal/remediate/rollback.go` with implementation

4. **Update Controller**: Add case in `operator/internal/controller/remediation_controller.go`

5. **Add YAML Patcher**: Update `github-app/internal/yaml/patcher.go` to handle new action

6. **Write Tests**: Add test cases for new functionality

7. **Update Docs**: Document the new remediation type

## Pull Request Process

1. **Before Opening PR**:
   - Run tests: `make test`
   - Run linters: `make fmt && make vet`
   - Update documentation if needed
   - Add tests for new features

2. **PR Description**:
   - Clearly describe what the PR does
   - Reference any related issues
   - Include screenshots/examples if relevant
   - List any breaking changes

3. **Review Process**:
   - At least one maintainer approval required
   - All CI checks must pass
   - Address review comments
   - Keep PR focused and reasonably sized

4. **After Merge**:
   - Delete your feature branch
   - Update your fork

## Testing

### Unit Tests

```bash
# Test specific package
go test ./internal/remediate/...

# Test with coverage
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

### Integration Tests

```bash
# Start Kind cluster
make kind-up

# Install CRDs
make install-operator

# Run integration tests
go test ./test/integration/...
```

### Manual Testing

Use the test application to trigger real scenarios:

```bash
kubectl apply -f examples/test-app/oom-deployment.yaml
```

## Documentation

- Keep README.md up to date
- Document new features in `docs/`
- Add code comments for complex logic
- Update architecture diagrams if structure changes

## Release Process

(For maintainers)

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create release branch
4. Run full test suite
5. Create GitHub release with notes
6. Build and push Docker images
7. Update Helm chart version

## Getting Help

- Open an issue for bugs or feature requests
- Join discussions in GitHub Discussions
- Ask questions in issues with `question` label

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

## Recognition

Contributors will be recognized in:
- README.md contributors section
- Release notes
- Project website (when available)

Thank you for contributing to heal8s!
