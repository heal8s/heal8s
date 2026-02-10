# Operator image for Docker and Kind (override with make verify IMG=my/operator:v1)
IMG ?= heal8s/operator:dev
KIND_NAME ?= heal8s-dev
# CI image for fully isolated runs (no Go/Helm/Kind on host)
CI_IMAGE ?= heal8s/ci:local

# Load Docker connection from .env (copy from .env.example, put token there if needed)
ifneq (,$(wildcard .env))
include .env
export DOCKER_HOST DOCKER_TLS_VERIFY DOCKER_CERT_PATH
export DOCKER_REGISTRY DOCKER_REGISTRY_USER DOCKER_REGISTRY_TOKEN
endif

.PHONY: help operator github-app kind-up kind-down test test-unit lint docker-operator kind-load-operator deploy-operator-helm verify e2e install-operator run-operator deploy-operator build-all clean ci-image test-unit-in-docker lint-in-docker verify-in-docker all-in-docker

help:
	@echo "heal8s Makefile targets:"
	@echo "  --- Isolated (all inside Docker, nothing on host except Docker) ---"
	@echo "  make ci-image            - Build CI image (Go, Helm, kubectl, Kind, Docker CLI)"
	@echo "  make test-unit-in-docker - Unit tests inside CI container"
	@echo "  make lint-in-docker      - Lint inside CI container"
	@echo "  make verify-in-docker    - Full verify inside CI container (build+Kind+deploy+assert)"
	@echo "  make all-in-docker      - Build CI image and run verify-in-docker"
	@echo "  --- Local (requires Go, Helm, Kind on host) ---"
	@echo "  make test-unit         - Run unit tests (operator + github-app)"
	@echo "  make lint              - Run go fmt/vet and helm lint"
	@echo "  make verify            - Full pipeline: test-unit, lint, build image, Kind, deploy, assert"
	@echo "  make e2e               - Run E2E script (examples/e2e-test.sh)"
	@echo "  make operator          - Build operator binary"
	@echo "  make github-app        - Build GitHub App service binary"
	@echo "  make build-all         - Build all components"
	@echo "  make docker-operator   - Build operator Docker image (IMG=$(IMG))"
	@echo "  make kind-up           - Start Kind cluster for development"
	@echo "  make kind-down         - Stop Kind cluster"
	@echo "  make kind-load-operator - Load operator image into Kind"
	@echo "  make install-operator  - Install CRDs to cluster"
	@echo "  make deploy-operator-helm - Deploy operator via Helm (requires image loaded)"
	@echo "  make run-operator      - Run operator locally (requires cluster)"
	@echo "  make test              - Alias for test-unit"
	@echo "  make clean             - Clean build artifacts"

## Unit tests (no cluster). Operator uses hand-written deepcopy.go (no controller-gen).
test-unit:
	@echo "Running operator tests..."
	cd operator && go test ./...
	@echo "Running GitHub App tests..."
	cd github-app && go test ./...

test: test-unit

## Lint: fmt, vet, helm lint
lint:
	@echo "Linting operator..."
	cd operator && go fmt ./... && go vet ./...
	@echo "Linting github-app..."
	cd github-app && go fmt ./... && go vet ./...
	@echo "Linting Helm chart..."
	helm lint charts/heal8s

## Build operator Docker image
docker-operator:
	cd operator && $(MAKE) docker-build IMG=$(IMG)

## Load operator image into Kind (run after kind-up and docker-operator)
kind-load-operator:
	kind load docker-image $(IMG) --name $(KIND_NAME)

## Deploy operator via Helm (CRDs + release). Use after kind-load-operator.
deploy-operator-helm:
	kubectl apply -f operator/config/crd/remediation-crd.yaml
	helm upgrade --install heal8s charts/heal8s -n heal8s-system --create-namespace \
		--set operator.image.repository=$(firstword $(subst :, ,$(IMG))) \
		--set operator.image.tag=$(if $(findstring :,$(IMG)),$(lastword $(subst :, ,$(IMG))),latest) \
		--set operator.image.pullPolicy=IfNotPresent \
		--wait --timeout=120s

## Full verify: test-unit -> lint -> docker-operator -> scripts/verify.sh
verify: test-unit lint docker-operator
	IMG=$(IMG) KIND_NAME=$(KIND_NAME) ./scripts/verify.sh

## --- Isolated: everything runs inside containers (host only needs Docker) ---
ci-image:
	docker build -t $(CI_IMAGE) -f Dockerfile.ci .

test-unit-in-docker: ci-image
	docker run --rm -v "$(CURDIR)":/workspace -w /workspace $(CI_IMAGE) ./scripts/ci-run.sh test-unit

lint-in-docker: ci-image
	docker run --rm -v "$(CURDIR)":/workspace -w /workspace $(CI_IMAGE) ./scripts/ci-run.sh lint

# For remote Docker (DOCKER_HOST=tcp://...) socket mount is omitted
DOCKER_SOCK_MOUNT := $(if $(filter tcp://%,$(DOCKER_HOST)),,-v /var/run/docker.sock:/var/run/docker.sock)
# Host network so kubectl inside container can reach Kind API server (bound on host)
verify-in-docker: ci-image
	docker run --rm --network host -v "$(CURDIR)":/workspace $(DOCKER_SOCK_MOUNT) -w /workspace \
		-e IMG=$(IMG) -e KIND_NAME=$(KIND_NAME) \
		-e DOCKER_HOST -e DOCKER_TLS_VERIFY -e DOCKER_CERT_PATH \
		-e DOCKER_REGISTRY -e DOCKER_REGISTRY_USER -e DOCKER_REGISTRY_TOKEN \
		$(CI_IMAGE) ./scripts/ci-run.sh verify

all-in-docker: verify-in-docker

## E2E: run e2e script (local operator or in-cluster; see script)
e2e:
	./examples/e2e-test.sh

operator:
	cd operator && make build

github-app:
	cd github-app && go build -o bin/github-app ./cmd/server

build-all: operator github-app

kind-up:
	@echo "Creating Kind cluster..."
	@kind create cluster --config examples/kind/cluster-config.yaml --name $(KIND_NAME) || echo "Cluster already exists"
	@kubectl cluster-info --context kind-$(KIND_NAME)

kind-down:
	@echo "Deleting Kind cluster..."
	kind delete cluster --name $(KIND_NAME)

install-operator:
	cd operator && make install

run-operator:
	cd operator && make run

deploy-operator: deploy-operator-helm
	@echo "Deploy operator: run 'make docker-operator kind-up kind-load-operator deploy-operator-helm' or 'make verify'"

clean:
	cd operator && make clean || true
	rm -rf github-app/bin
	rm -rf dist/
