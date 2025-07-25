# Proto parameters
PROTOC=protoc
PROTO_DIR=api
PROTO_OUT_DIR=.
MODULE_NAME=github.com/xuezhaojun/multiclustertunnel

# Tool versions - pinned for reproducible builds
PROTOC_GEN_GO_VERSION=v1.36.6
PROTOC_GEN_GO_GRPC_VERSION=v1.5.1

# Find all proto files under api/
PROTO_FILES=$(shell find $(PROTO_DIR) -name "*.proto")

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# Install required tools for proto generation
.PHONY: install-tools
install-tools: ## Install pinned versions of protoc plugins for Go
	@echo "Installing protoc plugins (pinned versions)..."
	@echo "  protoc-gen-go: $(PROTOC_GEN_GO_VERSION)"
	@echo "  protoc-gen-go-grpc: $(PROTOC_GEN_GO_GRPC_VERSION)"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@echo "Tools installed successfully!"

# Generate Go code from proto files
.PHONY: proto-gen
proto-gen: install-tools ## Generate Go code from proto files
	@echo "Generating Go code from proto files..."
	$(PROTOC) \
		--go_out=$(PROTO_OUT_DIR) \
		--go_opt=module=$(MODULE_NAME) \
		--go-grpc_out=$(PROTO_OUT_DIR) \
		--go-grpc_opt=module=$(MODULE_NAME) \
		--proto_path=$(PROTO_DIR) \
		$(PROTO_FILES)
	@echo "Proto generation completed!"

# Build targets
.PHONY: build-server
build-server:
	@mkdir -p _output
	go build -o _output/test-hub ./cmd/test-hub/

.PHONY: build-agent
build-agent:
	@mkdir -p _output
	go build -o _output/test-agent ./cmd/test-agent/

.PHONY: build-all
build-all: build-server build-agent ## Build all binaries

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf _output/

# Dependency management
.PHONY: deps-tidy
deps-tidy: ## Tidy and verify dependencies
	go mod tidy
	go mod verify

.PHONY: deps-update
deps-update: ## Update dependencies to latest versions
	go get -u ./...
	go mod tidy

# Quick unit tests for development.
.PHONY: test-ut
test-ut: ## Run unit tests
	go test -race $(shell go list ./... | grep -v -e "/tests$$" -e "/tests/.*")

.PHONY: test
test: test-ut ## Run unit tests (alias for test-ut)

.PHONY: test-integration
test-integration: ## Run integration tests
	go test -race -v ./tests/integration

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	go test -race ./tests/e2e

.PHONY: test-all
test-all: test-ut test-integration test-e2e ## Run all tests
