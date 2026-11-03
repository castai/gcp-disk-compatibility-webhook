LOCALBIN ?= $(shell pwd)/bin
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= $(shell cat .golangci-lint-version)

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the webhook binary into bin/.
	go build -o bin/webhook .

.PHONY: test
test: ## Run unit tests with the race detector.
	go test -race ./...

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

.PHONY: clean
clean: ## Remove build artifacts.
	rm -rf $(LOCALBIN)

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

$(GOLANGCI_LINT): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
