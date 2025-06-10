# Proto parameters
PROTOC=protoc
PROTO_DIR=api
PROTO_OUT_DIR=api
MODULE_NAME=github.com/xuezhaojun/multiclustertunnel

# Find all proto files
PROTO_FILES=$(shell find $(PROTO_DIR) -name "*.proto")

# Install required tools for proto generation
.PHONY: install-tools
install-tools:
	@echo "Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	@echo "Tools installed successfully!"

# Generate Go code from proto files
.PHONY: proto-gen
proto-gen: install-tools
	@echo "Generating Go code from proto files..."
	@for proto in $(PROTO_FILES); do \
		echo "Processing $$proto..."; \
		$(PROTOC) \
			--go_out=$(PROTO_OUT_DIR) \
			--go_opt=module=$(MODULE_NAME) \
			--go-grpc_out=$(PROTO_OUT_DIR) \
			--go-grpc_opt=module=$(MODULE_NAME) \
			--proto_path=$(PROTO_DIR) \
			$$proto; \
	done
	@echo "Proto generation completed!"

# Clean generated proto files
.PHONY: proto-clean
proto-clean:
	@echo "Cleaning generated proto files..."
	@find $(PROTO_OUT_DIR) -name "*.pb.go" -delete
	@find $(PROTO_OUT_DIR) -name "*_grpc.pb.go" -delete
	@echo "Clean completed!"