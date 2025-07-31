#!/bin/bash

# Build script for MultiClusterTunnel E2E Docker images
# This script builds both server and agent images for e2e testing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVER_IMAGE_NAME="${SERVER_IMAGE_NAME:-mctunnel-server}"
AGENT_IMAGE_NAME="${AGENT_IMAGE_NAME:-mctunnel-agent}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
BUILD_ARGS="${BUILD_ARGS:-}"
PUSH_IMAGES="${PUSH_IMAGES:-false}"
REGISTRY="${REGISTRY:-}"


echo -e "${BLUE}=== MultiClusterTunnel E2E Image Builder ===${NC}"
echo ""

# Function to check if Docker is running
check_docker() {
    if ! docker info &> /dev/null; then
        echo -e "${RED}Error: Docker is not running${NC}"
        echo "Please start Docker daemon"
        exit 1
    fi
}

# Function to build an image
build_image() {
    local dockerfile=$1
    local image_name=$2
    local context=${3:-.}

    echo -e "${BLUE}Building ${image_name}:${IMAGE_TAG}...${NC}"
    echo -e "${YELLOW}Dockerfile: ${dockerfile}${NC}"
    echo -e "${YELLOW}Context: ${context}${NC}"

    # Build the image
    if docker build \
        -f "${dockerfile}" \
        -t "${image_name}:${IMAGE_TAG}" \
        ${BUILD_ARGS} \
        "${context}"; then
        echo -e "${GREEN}✓ Successfully built ${image_name}:${IMAGE_TAG}${NC}"
    else
        echo -e "${RED}✗ Failed to build ${image_name}:${IMAGE_TAG}${NC}"
        exit 1
    fi

    # Tag with registry if specified
    if [ -n "${REGISTRY}" ]; then
        local registry_tag="${REGISTRY}/${image_name}:${IMAGE_TAG}"
        docker tag "${image_name}:${IMAGE_TAG}" "${registry_tag}"
        echo -e "${GREEN}✓ Tagged as ${registry_tag}${NC}"
    fi

    echo ""
}

# Function to push images
push_image() {
    local image_name=$1

    if [ "${PUSH_IMAGES}" = "true" ]; then
        echo -e "${BLUE}Pushing ${image_name}:${IMAGE_TAG}...${NC}"

        if [ -n "${REGISTRY}" ]; then
            local registry_tag="${REGISTRY}/${image_name}:${IMAGE_TAG}"
            if docker push "${registry_tag}"; then
                echo -e "${GREEN}✓ Successfully pushed ${registry_tag}${NC}"
            else
                echo -e "${RED}✗ Failed to push ${registry_tag}${NC}"
                exit 1
            fi
        else
            if docker push "${image_name}:${IMAGE_TAG}"; then
                echo -e "${GREEN}✓ Successfully pushed ${image_name}:${IMAGE_TAG}${NC}"
            else
                echo -e "${RED}✗ Failed to push ${image_name}:${IMAGE_TAG}${NC}"
                exit 1
            fi
        fi
        echo ""
    fi
}

# Function to show image info
show_image_info() {
    local image_name=$1
    echo -e "${BLUE}Image information for ${image_name}:${IMAGE_TAG}:${NC}"
    docker images "${image_name}:${IMAGE_TAG}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
    echo ""
}

# Main execution
main() {
    echo -e "${YELLOW}Configuration:${NC}"
    echo -e "  Server Image: ${SERVER_IMAGE_NAME}:${IMAGE_TAG}"
    echo -e "  Agent Image: ${AGENT_IMAGE_NAME}:${IMAGE_TAG}"
    echo -e "  Agent Variant: ${AGENT_VARIANT}"
    echo -e "  Registry: ${REGISTRY:-<none>}"
    echo -e "  Push Images: ${PUSH_IMAGES}"
    echo -e "  Build Args: ${BUILD_ARGS:-<none>}"
    echo ""

    # Check prerequisites
    check_docker

    # Build server image
    build_image "build/server/Dockerfile" "${SERVER_IMAGE_NAME}"
    show_image_info "${SERVER_IMAGE_NAME}"
    push_image "${SERVER_IMAGE_NAME}"

    # Build agent image
    build_image "build/agent/Dockerfile" "${AGENT_IMAGE_NAME}"
    show_image_info "${AGENT_IMAGE_NAME}"
    push_image "${AGENT_IMAGE_NAME}"

    echo -e "${GREEN}=== Build completed successfully! ===${NC}"
    echo ""
    echo -e "${BLUE}Built images:${NC}"
    echo -e "  • ${SERVER_IMAGE_NAME}:${IMAGE_TAG}"
    echo -e "  • ${AGENT_IMAGE_NAME}:${IMAGE_TAG}"

    if [ -n "${REGISTRY}" ]; then
        echo ""
        echo -e "${BLUE}Registry images:${NC}"
        echo -e "  • ${REGISTRY}/${SERVER_IMAGE_NAME}:${IMAGE_TAG}"
        echo -e "  • ${REGISTRY}/${AGENT_IMAGE_NAME}:${IMAGE_TAG}"
    fi

    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo -e "  • Test images: docker-compose -f docker-compose.e2e.yml up"
    echo -e "  • Load into Kind: kind load docker-image ${SERVER_IMAGE_NAME}:${IMAGE_TAG} --name <cluster-name>"
    echo -e "  • Run e2e tests: make test-e2e-kind"
}

# Handle command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --server-image)
            SERVER_IMAGE_NAME="$2"
            shift 2
            ;;
        --agent-image)
            AGENT_IMAGE_NAME="$2"
            shift 2
            ;;
        --tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        --registry)
            REGISTRY="$2"
            shift 2
            ;;
        --push)
            PUSH_IMAGES="true"
            shift
            ;;
        --build-arg)
            BUILD_ARGS="${BUILD_ARGS} --build-arg $2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --server-image NAME    Server image name (default: mctunnel-server)"
            echo "  --agent-image NAME     Agent image name (default: mctunnel-agent)"
            echo "  --tag TAG              Image tag (default: latest)"
            echo "  --registry REGISTRY    Docker registry prefix"
            echo "  --push                 Push images to registry"
            echo "  --build-arg ARG=VALUE  Pass build argument to docker build"
            echo "  --help                 Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  SERVER_IMAGE_NAME      Server image name"
            echo "  AGENT_IMAGE_NAME       Agent image name"
            echo "  IMAGE_TAG              Image tag"
            echo "  REGISTRY               Docker registry prefix"
            echo "  PUSH_IMAGES            Push images (true/false)"
            echo "  BUILD_ARGS             Additional build arguments"
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
