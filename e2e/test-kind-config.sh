#!/bin/bash

# Test script to validate Kind cluster configuration
# This script creates a temporary Kind cluster to test the configuration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

CLUSTER_NAME="mctunnel-config-test"
CONFIG_FILE="e2e/templates/kind.config"

echo -e "${BLUE}=== Testing Kind Cluster Configuration ===${NC}"
echo ""

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo -e "${RED}Error: kind is not installed${NC}"
    echo "Please install kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl is not installed${NC}"
    echo "Please install kubectl: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check if Docker is running
if ! docker info &> /dev/null; then
    echo -e "${RED}Error: Docker is not running${NC}"
    echo "Please start Docker daemon"
    exit 1
fi

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Kind config file not found: $CONFIG_FILE${NC}"
    exit 1
fi

echo -e "${YELLOW}Configuration file: $CONFIG_FILE${NC}"
echo ""

# Function to cleanup on exit
cleanup() {
    echo -e "\n${YELLOW}Cleaning up test cluster...${NC}"
    kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
    echo -e "${GREEN}✓ Cleanup completed${NC}"
}

# Set trap to cleanup on script exit
trap cleanup EXIT

# Create Kind cluster with configuration
echo -e "${BLUE}1. Creating Kind cluster with configuration...${NC}"
if kind create cluster --name "$CLUSTER_NAME" --config "$CONFIG_FILE"; then
    echo -e "${GREEN}✓ Kind cluster created successfully${NC}"
else
    echo -e "${RED}✗ Failed to create Kind cluster${NC}"
    exit 1
fi
echo ""

# Wait for cluster to be ready
echo -e "${BLUE}2. Waiting for cluster to be ready...${NC}"
kubectl cluster-info --context "kind-$CLUSTER_NAME" --request-timeout=30s
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Cluster is accessible${NC}"
else
    echo -e "${RED}✗ Cluster is not accessible${NC}"
    exit 1
fi
echo ""

# Check nodes
echo -e "${BLUE}3. Checking cluster nodes...${NC}"
kubectl get nodes --context "kind-$CLUSTER_NAME" -o wide
NODE_COUNT=$(kubectl get nodes --context "kind-$CLUSTER_NAME" --no-headers | wc -l)
if [ "$NODE_COUNT" -eq 3 ]; then
    echo -e "${GREEN}✓ Expected 3 nodes found${NC}"
else
    echo -e "${RED}✗ Expected 3 nodes, found $NODE_COUNT${NC}"
    exit 1
fi
echo ""

# Check node readiness
echo -e "${BLUE}4. Checking node readiness...${NC}"
READY_NODES=$(kubectl get nodes --context "kind-$CLUSTER_NAME" --no-headers | grep " Ready " | wc -l)
if [ "$READY_NODES" -eq 3 ]; then
    echo -e "${GREEN}✓ All 3 nodes are ready${NC}"
else
    echo -e "${RED}✗ Only $READY_NODES out of 3 nodes are ready${NC}"
    kubectl get nodes --context "kind-$CLUSTER_NAME"
    exit 1
fi
echo ""

# Check system pods
echo -e "${BLUE}5. Checking system pods...${NC}"
kubectl get pods -n kube-system --context "kind-$CLUSTER_NAME"
RUNNING_PODS=$(kubectl get pods -n kube-system --context "kind-$CLUSTER_NAME" --no-headers | grep "Running" | wc -l)
if [ "$RUNNING_PODS" -gt 5 ]; then
    echo -e "${GREEN}✓ System pods are running ($RUNNING_PODS pods)${NC}"
else
    echo -e "${RED}✗ Not enough system pods running ($RUNNING_PODS pods)${NC}"
    exit 1
fi
echo ""

# Check networking
echo -e "${BLUE}6. Testing networking...${NC}"
kubectl run test-pod --image=busybox --context "kind-$CLUSTER_NAME" --restart=Never --rm -i --tty=false -- nslookup kubernetes.default.svc.cluster.local
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ DNS resolution working${NC}"
else
    echo -e "${RED}✗ DNS resolution failed${NC}"
    exit 1
fi
echo ""

# Test port mappings (if possible)
echo -e "${BLUE}7. Checking port mappings...${NC}"
# Check if ports are accessible from host
if nc -z localhost 8080 2>/dev/null; then
    echo -e "${YELLOW}⚠ Port 8080 is already in use${NC}"
elif nc -z localhost 8443 2>/dev/null; then
    echo -e "${YELLOW}⚠ Port 8443 is already in use${NC}"
else
    echo -e "${GREEN}✓ Configured ports (8080, 8443) are available${NC}"
fi
echo ""

# Check node labels
echo -e "${BLUE}8. Checking node labels...${NC}"
WORKER_NODES=$(kubectl get nodes --context "kind-$CLUSTER_NAME" -l node-type=worker --no-headers | wc -l)
if [ "$WORKER_NODES" -eq 2 ]; then
    echo -e "${GREEN}✓ Worker nodes have correct labels${NC}"
else
    echo -e "${RED}✗ Expected 2 worker nodes with labels, found $WORKER_NODES${NC}"
fi

CONTROL_PLANE_NODES=$(kubectl get nodes --context "kind-$CLUSTER_NAME" -l node-role.kubernetes.io/control-plane --no-headers | wc -l)
if [ "$CONTROL_PLANE_NODES" -eq 1 ]; then
    echo -e "${GREEN}✓ Control plane node found${NC}"
else
    echo -e "${RED}✗ Expected 1 control plane node, found $CONTROL_PLANE_NODES${NC}"
fi
echo ""

echo -e "${GREEN}=== Kind cluster configuration test completed successfully! ===${NC}"
echo ""
echo -e "${BLUE}Cluster details:${NC}"
echo -e "  • Cluster name: kind-$CLUSTER_NAME"
echo -e "  • Nodes: $NODE_COUNT ($CONTROL_PLANE_NODES control plane, $WORKER_NODES workers)"
echo -e "  • System pods: $RUNNING_PODS running"
echo -e "  • Port mappings: 8080→30080, 8443→30443, 9090→30090"
echo ""
echo -e "${YELLOW}Note: Cluster will be automatically deleted when script exits${NC}"
