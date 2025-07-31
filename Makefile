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

# Local testing build targets
.PHONY: build-test-server
build-test-server: ## Build test server binary
	@mkdir -p _output
	go build -o _output/test-server ./cmd/test-server/

.PHONY: build-test-agent
build-test-agent: ## Build test agent binary
	@mkdir -p _output
	go build -o _output/test-agent ./cmd/test-agent/

.PHONY: build-test-simple-server
build-test-simple-server: ## Build test simple server binary
	@mkdir -p _output
	go build -o _output/test-simple-server ./cmd/test-simple-server/

.PHONY: build-local-test
build-local-test: build-test-server build-test-agent build-test-simple-server ## Build all local testing binaries

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

# Local testing targets
.PHONY: start-test-env
start-test-env: build-local-test ## Start the local test environment
	@echo "Starting local test environment..."
	./scripts/start-test-env.sh

.PHONY: test-tunnel
test-tunnel: ## Run tunnel functionality tests (requires services to be running)
	@echo "Running tunnel tests..."
	./scripts/test-tunnel.sh

# E2E testing targets

# Certificate generation
.PHONY: make-certs
make-certs: ## Generate certificates for e2e testing
	@echo "Generating certificates for e2e testing..."
	@mkdir -p _output
	go build -o _output/generate-certs ./cmd/generate-certs/
	./_output/generate-certs --output-dir=e2e/certs
	@echo "Certificates generated successfully!"

# E2E Docker images
.PHONY: build-e2e-images
build-e2e-images: ## Build Docker images for e2e testing
	@echo "Building e2e Docker images..."
	./scripts/build-e2e-images.sh
	@echo "E2E Docker images built successfully!"

.PHONY: build-e2e-images-push
build-e2e-images-push: ## Build and push Docker images for e2e testing
	@echo "Building and pushing e2e Docker images..."
	PUSH_IMAGES=true ./scripts/build-e2e-images.sh
	@echo "E2E Docker images built and pushed successfully!"

.PHONY: build-server-image
build-server-image: ## Build server Docker image
	@echo "Building server Docker image..."
	docker build -f build/server/Dockerfile -t mctunnel-server:latest .
	@echo "Server Docker image built successfully!"

.PHONY: build-agent-image
build-agent-image: ## Build agent Docker image
	@echo "Building agent Docker image..."
	docker build -f build/agent/Dockerfile -t mctunnel-agent:latest .
	@echo "Agent Docker image built successfully!"

.PHONY: test-docker-compose
test-docker-compose: build-e2e-images ## Test images with Docker Compose
	@echo "Testing e2e images with Docker Compose..."
	docker-compose -f docker-compose.e2e.yml up --build -d
	@echo "Docker Compose services started. Check with: docker-compose -f docker-compose.e2e.yml ps"

.PHONY: stop-docker-compose
stop-docker-compose: ## Stop Docker Compose services
	@echo "Stopping Docker Compose services..."
	docker-compose -f docker-compose.e2e.yml down -v
	@echo "Docker Compose services stopped!"

# E2E testing
.PHONY: test-e2e-kind
test-e2e-kind: build-e2e-images make-certs ## Run e2e tests with Kind cluster
	@echo "Running e2e tests with Kind cluster..."
	go test -v ./e2e -timeout 30m \
		-server-image=mctunnel-server:latest \
		-agent-image=mctunnel-agent:latest \
		-kind-image=kindest/node:v1.30.2

.PHONY: test-e2e-kind-ci
test-e2e-kind-ci: ## Run e2e tests in CI (assumes images are already built)
	@echo "Running e2e tests in CI..."
	go test -v ./e2e -timeout 30m \
		-server-image=${SERVER_IMAGE:-mctunnel-server:latest} \
		-agent-image=${AGENT_IMAGE:-mctunnel-agent:latest} \
		-kind-image=${KIND_IMAGE:-kindest/node:v1.30.2}

# E2E utilities
.PHONY: test-kind-config
test-kind-config: ## Test Kind cluster configuration
	@echo "Testing Kind cluster configuration..."
	./e2e/test-kind-config.sh

.PHONY: clean-e2e
clean-e2e: ## Clean e2e artifacts
	@echo "Cleaning e2e artifacts..."
	rm -rf e2e/certs/
	kind delete cluster --name mctunnel-e2e 2>/dev/null || true
	@echo "E2E cleanup completed!"
