#!/bin/bash

# Script to load Docker images into Kind cluster for e2e testing
# This script loads the built images into the specified Kind cluster

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-mctunnel-e2e}"
SERVER_IMAGE="${SERVER_IMAGE:-mctunnel-server:latest}"
AGENT_IMAGE="${AGENT_IMAGE:-mctunnel-agent:latest}"
BUILD_IMAGES="${BUILD_IMAGES:-false}"

echo -e "${BLUE}=== Kind Image Loader for MultiClusterTunnel E2E ===${NC}"
echo ""

# Function to check if kind is installed
check_kind() {
    if ! command -v kind &> /dev/null; then
        echo -e "${RED}Error: kind is not installed${NC}"
        echo "Please install kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        exit 1
    fi
}

# Function to check if Docker is running
check_docker() {
    if ! docker info &> /dev/null; then
        echo -e "${RED}Error: Docker is not running${NC}"
        echo "Please start Docker daemon"
        exit 1
    fi
}

# Function to check if cluster exists
check_cluster() {
    if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        echo -e "${RED}Error: Kind cluster '${CLUSTER_NAME}' does not exist${NC}"
        echo "Available clusters:"
        kind get clusters | sed 's/^/  • /'
        echo ""
        echo "Create cluster with: kind create cluster --name ${CLUSTER_NAME}"
        exit 1
    fi
}

# Function to check if image exists locally
check_image() {
    local image=$1
    if ! docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${image}$"; then
        echo -e "${YELLOW}Warning: Image '${image}' not found locally${NC}"
        return 1
    fi
    return 0
}

# Function to load image into kind cluster
load_image() {
    local image=$1
    local image_name=$(echo "$image" | cut -d':' -f1)
    
    echo -e "${BLUE}Loading ${image} into cluster ${CLUSTER_NAME}...${NC}"
    
    if kind load docker-image "${image}" --name "${CLUSTER_NAME}"; then
        echo -e "${GREEN}✓ Successfully loaded ${image}${NC}"
    else
        echo -e "${RED}✗ Failed to load ${image}${NC}"
        exit 1
    fi
    
    # Verify the image is available in the cluster
    echo -e "${YELLOW}Verifying image in cluster...${NC}"
    if docker exec "${CLUSTER_NAME}-control-plane" crictl images | grep -q "${image_name}"; then
        echo -e "${GREEN}✓ Image ${image} is available in cluster${NC}"
    else
        echo -e "${YELLOW}⚠ Image verification failed, but this might be normal${NC}"
    fi
    echo ""
}

# Function to build images if requested
build_images() {
    if [ "${BUILD_IMAGES}" = "true" ]; then
        echo -e "${BLUE}Building images before loading...${NC}"
        if [ -f "scripts/build-e2e-images.sh" ]; then
            ./scripts/build-e2e-images.sh
        else
            echo -e "${YELLOW}Build script not found, building with make...${NC}"
            make build-e2e-images
        fi
        echo ""
    fi
}

# Function to show cluster info
show_cluster_info() {
    echo -e "${BLUE}Cluster information:${NC}"
    echo -e "  • Cluster name: ${CLUSTER_NAME}"
    echo -e "  • Cluster status: $(kind get clusters | grep "^${CLUSTER_NAME}$" && echo "Running" || echo "Not found")"
    
    # Show nodes
    echo -e "  • Nodes:"
    kubectl get nodes --context "kind-${CLUSTER_NAME}" 2>/dev/null | tail -n +2 | sed 's/^/    /'
    
    # Show images in cluster
    echo -e "  • Images in cluster:"
    docker exec "${CLUSTER_NAME}-control-plane" crictl images 2>/dev/null | grep -E "(mctunnel|multiclustertunnel)" | sed 's/^/    /' || echo "    No mctunnel images found"
    echo ""
}

# Main execution
main() {
    echo -e "${YELLOW}Configuration:${NC}"
    echo -e "  • Cluster Name: ${CLUSTER_NAME}"
    echo -e "  • Server Image: ${SERVER_IMAGE}"
    echo -e "  • Agent Image: ${AGENT_IMAGE}"
    echo -e "  • Build Images: ${BUILD_IMAGES}"
    echo ""
    
    # Check prerequisites
    check_kind
    check_docker
    check_cluster
    
    # Build images if requested
    build_images
    
    # Check if images exist locally
    local missing_images=()
    if ! check_image "${SERVER_IMAGE}"; then
        missing_images+=("${SERVER_IMAGE}")
    fi
    if ! check_image "${AGENT_IMAGE}"; then
        missing_images+=("${AGENT_IMAGE}")
    fi
    
    if [ ${#missing_images[@]} -gt 0 ]; then
        echo -e "${RED}Missing images:${NC}"
        for img in "${missing_images[@]}"; do
            echo -e "  • ${img}"
        done
        echo ""
        echo -e "${YELLOW}Build images with: make build-e2e-images${NC}"
        echo -e "${YELLOW}Or run with --build flag to build automatically${NC}"
        exit 1
    fi
    
    # Load images into cluster
    load_image "${SERVER_IMAGE}"
    load_image "${AGENT_IMAGE}"
    
    # Show cluster information
    show_cluster_info
    
    echo -e "${GREEN}=== Images loaded successfully! ===${NC}"
    echo ""
    echo -e "${BLUE}Next steps:${NC}"
    echo -e "  • Run e2e tests: make test-e2e-kind"
    echo -e "  • Check cluster: kubectl get pods -A --context kind-${CLUSTER_NAME}"
    echo -e "  • Delete cluster: kind delete cluster --name ${CLUSTER_NAME}"
}

# Handle command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --server-image)
            SERVER_IMAGE="$2"
            shift 2
            ;;
        --agent-image)
            AGENT_IMAGE="$2"
            shift 2
            ;;
        --build)
            BUILD_IMAGES="true"
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --cluster-name NAME    Kind cluster name (default: mctunnel-e2e)"
            echo "  --server-image IMAGE   Server image to load (default: mctunnel-server:latest)"
            echo "  --agent-image IMAGE    Agent image to load (default: mctunnel-agent:latest)"
            echo "  --build                Build images before loading"
            echo "  --help                 Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  CLUSTER_NAME           Kind cluster name"
            echo "  SERVER_IMAGE           Server image name"
            echo "  AGENT_IMAGE            Agent image name"
            echo "  BUILD_IMAGES           Build images before loading (true/false)"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main
