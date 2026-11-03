LOCALBIN ?= $(shell pwd)/bin
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= $(shell cat .golangci-lint-version)

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

# Build the webhook binary (linux/amd64, suitable for containers/kind).
.PHONY: build
build: ## Build the webhook binary (linux/amd64) as ./webhook.
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o webhook main.go

# Native host build, handy for quick local runs/tests.
.PHONY: build-local
build-local: ## Build a native binary into bin/webhook.
	go build -o $(LOCALBIN)/webhook .

# Build the Docker image
.PHONY: docker-build
docker-build: ## Build the Docker image (disk-compatibility-webhook:latest).
	docker build -t disk-compatibility-webhook:latest .

.PHONY: kind-load
kind-load: docker-build ## Load the image into the local kind cluster (kwok).
	kind load docker-image disk-compatibility-webhook -n kwok

.PHONY: test
test: ## Run unit tests (race detector).
	go test -v -race ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: fmt
fmt: ## Format code with gofmt -s.
	gofmt -s -w .

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint (downloads the pinned version if needed).
	$(GOLANGCI_LINT) run --timeout=10m

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Run golangci-lint with automatic fixes.
	$(GOLANGCI_LINT) run --fix --timeout=10m

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum.
	go mod tidy

# Remove build artifacts. (The old kubectl teardown targets were dropped
# upstream together with deploy/ — a0baad8; deployment now goes through Helm.)
.PHONY: clean
clean: ## Remove build artifacts (./webhook and bin/).
	rm -rf webhook $(LOCALBIN)

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

$(GOLANGCI_LINT): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
