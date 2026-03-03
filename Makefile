# Makefile for secure-inference

# Build params
PROJECT_NAME ?= secure-inference
CGO_ENABLED ?= 0
export GO111MODULE=on

# Container runtime
CONTAINER_RUNTIME ?= docker

# Image names
REGISTRY ?= ghcr.io/llm-d
IMAGE ?= $(REGISTRY)/$(PROJECT_NAME)
IMG ?= $(PROJECT_NAME):latest
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PLATFORMS ?= linux/amd64
# Sidecars to build/load: comma-separated <folder>=<image> pairs
# Example: SIDECARS=adapter-selection=adapter-selection:latest
SIDECARS ?=

# Helpers to parse SIDECARS entries
sidecar_dir = $(word 1,$(subst =, ,$(1)))
sidecar_img = $(word 2,$(subst =, ,$(1)))
comma := ,

# Go configuration
GOFLAGS ?=
LDFLAGS ?= -s -w -X main.version=$(VERSION)

# Directories
LOCALBIN ?= $(shell pwd)/bin
CHART_DIR = ./charts/secure-inference

# Demo/deployment env variables
MINIKUBE_PROFILE ?= minikube
GATEWAY_TYPE ?= istio
K8S_NAMESPACE ?= llm-d-inference-scheduler
LLMD_HTTP_ROUTE ?= llm-d-inference-scheduling
LLMD_GATEWAY_NAME ?= infra-inference-scheduling-inference-gateway

# Tools
GOLANGCI_LINT_VERSION ?= v2.10.1
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.18.0
ENVTEST ?= $(LOCALBIN)/setup-envtest
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Format Go and Python code
	gofmt -w .
	@if ls *.py **/*.py 2>/dev/null | head -1 > /dev/null 2>&1; then \
		ruff format .; \
	fi

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: lint
lint: lint-go lint-python ## Run all linters

.PHONY: lint-go
lint-go: ## Run Go linter (golangci-lint v2)
	golangci-lint run

.PHONY: lint-python
lint-python: ## Run Python linter (ruff) — skipped if no Python files found
	@if ls *.py **/*.py 2>/dev/null | head -1 > /dev/null 2>&1; then \
		ruff check . && ruff format --check .; \
	else \
		echo "No Python files found, skipping Python lint"; \
	fi

.PHONY: pre-commit
pre-commit: ## Run pre-commit hooks on all files
	pre-commit run --all-files

##@ Code Generation

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(LOCALBIN)/controller-gen || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./api/..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code (DeepCopy)
	$(CONTROLLER_GEN) object paths="./api/..."

.PHONY: setup-envtest
setup-envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	@test -s $(LOCALBIN)/setup-envtest || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

##@ Build

.PHONY: build
build: ## Build secure-inference binary
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(PROJECT_NAME) ./cmd/secure-inference

.PHONY: build-cli
build-cli: ## Build llmd-admin CLI binary
	CGO_ENABLED=$(CGO_ENABLED) go build -o bin/llmd-admin ./cmd/llmd-admin

.PHONY: build-deployment-customizer
build-deployment-customizer: ## Build deployment-customizer binary
	CGO_ENABLED=$(CGO_ENABLED) go build -o bin/deployment-customizer ./cmd/deployment-customizer

.PHONY: build-all
build-all: build build-cli build-deployment-customizer ## Build all binaries

##@ Docker Images

.PHONY: image-build
image-build: ## Build Docker images (main + SIDECARS)
	$(CONTAINER_RUNTIME) build --platform $(PLATFORMS) --rm --tag $(IMG) -f Dockerfile .
	$(foreach s,$(subst $(comma), ,$(SIDECARS)), \
		$(CONTAINER_RUNTIME) build --rm \
			--tag $(call sidecar_img,$(s)) \
			-f sidecar/$(call sidecar_dir,$(s))/Dockerfile \
			sidecar/$(call sidecar_dir,$(s)) ;)

.PHONY: image-push
image-push: ## Build and push multi-arch container image
	$(CONTAINER_RUNTIME) buildx build \
		--platform $(PLATFORMS) \
		--push \
		--annotation "index:org.opencontainers.image.source=https://github.com/llm-d/$(PROJECT_NAME)" \
		--annotation "index:org.opencontainers.image.licenses=Apache-2.0" \
		--tag $(IMAGE):$(VERSION) \
		--tag $(IMAGE):latest \
		.

##@ Testing

.PHONY: test
test: setup-envtest ## Run unit tests
	KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN)/k8s -p path)" \
	go test -race -count=1 ./pkg/...

.PHONY: test-coverage
test-coverage: setup-envtest ## Run tests with coverage report
	KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN)/k8s -p path)" \
	go test -race -coverprofile=coverage.out -covermode=atomic ./pkg/...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	go test -race -count=1 ./test/e2e/... -v

.PHONY: test-all
test-all: test test-e2e ## Run all tests

##@ Deployment

.PHONY: install-crds
install-crds: manifests ## Install CRDs to cluster
	kubectl apply -k config/crd

.PHONY: uninstall-crds
uninstall-crds: ## Uninstall CRDs from cluster
	kubectl delete -k config/crd

HELM_VALUES ?=
HELM_SETS = --set image=$(IMG) \
	--set gateway=$(GATEWAY_TYPE) \
	--set target_http_route=$(LLMD_HTTP_ROUTE) \
	--set gateway_name=$(LLMD_GATEWAY_NAME)

.PHONY: deploy-helm
deploy-helm: ## Deploy using Helm chart
	helm install $(PROJECT_NAME) $(CHART_DIR) \
		$(if $(HELM_VALUES),-f $(HELM_VALUES),$(HELM_SETS)) \
		--namespace $(K8S_NAMESPACE)

.PHONY: upgrade-helm
upgrade-helm: ## Upgrade Helm deployment
	helm upgrade $(PROJECT_NAME) $(CHART_DIR) \
		$(if $(HELM_VALUES),-f $(HELM_VALUES),$(HELM_SETS)) \
		--namespace $(K8S_NAMESPACE)

.PHONY: uninstall-helm
uninstall-helm: ## Uninstall Helm deployment
	helm uninstall $(PROJECT_NAME) --namespace $(K8S_NAMESPACE)

##@ Minikube

.PHONY: load-images-minikube
load-images-minikube: ## Load images to Minikube (main + SIDECARS)
	minikube image load $(IMG) -p $(MINIKUBE_PROFILE)
	$(foreach s,$(subst $(comma), ,$(SIDECARS)), \
		minikube image load $(call sidecar_img,$(s)) -p $(MINIKUBE_PROFILE) ;)

##@ Full Deployment

.PHONY: deploy
deploy: ## Deploy all components to cluster
	./bin/llmd-admin init
	kubectl create secret generic llm-d-access --from-file=publickey=./certs/llm-d-ca.pem -n $(K8S_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -k config/crd
	$(MAKE) deploy-helm

##@ CI Helpers

.PHONY: ci-lint
ci-lint: ## CI: install and run golangci-lint
	@which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	golangci-lint run

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/ coverage.out coverage.html
