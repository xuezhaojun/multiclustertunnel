#!/bin/bash

# Script to start the test environment for multiclustertunnel
# This script starts all three components in the correct order and provides testing instructions

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SIMPLE_SERVER_PORT=9090
HUB_SERVER_GRPC_PORT=8443
HUB_SERVER_HTTP_PORT=8080

echo -e "${BLUE}=== MultiCluster Tunnel Test Environment Setup ===${NC}"
echo ""

# Check if binaries exist
if [ ! -f "_output/test-simple-server" ] || [ ! -f "_output/test-server" ] || [ ! -f "_output/test-agent" ]; then
    echo -e "${YELLOW}Building binaries...${NC}"
    make build-local-test
    echo -e "${GREEN}✓ Binaries built successfully${NC}"
    echo ""
fi

# Function to cleanup processes on exit
cleanup() {
    echo -e "\n${YELLOW}Cleaning up processes...${NC}"
    pkill -f "test-simple-server" 2>/dev/null || true
    pkill -f "test-server" 2>/dev/null || true
    pkill -f "test-agent" 2>/dev/null || true
    echo -e "${GREEN}✓ Cleanup completed${NC}"
}

# Set trap to cleanup on script exit
trap cleanup EXIT

# Start test-simple-server
echo -e "${BLUE}1. Starting test-simple-server on port ${SIMPLE_SERVER_PORT}...${NC}"
./_output/test-simple-server -addr=":${SIMPLE_SERVER_PORT}" -v=2 &
SIMPLE_SERVER_PID=$!
sleep 2

# Check if simple server is running
if ! curl -s http://localhost:${SIMPLE_SERVER_PORT}/health > /dev/null; then
    echo -e "${RED}✗ Failed to start test-simple-server${NC}"
    exit 1
fi
echo -e "${GREEN}✓ test-simple-server started (PID: ${SIMPLE_SERVER_PID})${NC}"
echo ""

# Start test-server (hub)
echo -e "${BLUE}2. Starting test-server (hub) on gRPC port ${HUB_SERVER_GRPC_PORT} and HTTP port ${HUB_SERVER_HTTP_PORT}...${NC}"
./_output/test-server -grpc-addr=":${HUB_SERVER_GRPC_PORT}" -http-addr=":${HUB_SERVER_HTTP_PORT}" -v=2 &
HUB_SERVER_PID=$!
sleep 3

# Check if hub server is running
if ! curl -s http://localhost:${HUB_SERVER_HTTP_PORT}/health > /dev/null; then
    echo -e "${RED}✗ Failed to start test-server${NC}"
    exit 1
fi
echo -e "${GREEN}✓ test-server started (PID: ${HUB_SERVER_PID})${NC}"
echo ""

# Start test-agent
echo -e "${BLUE}3. Starting test-agent...${NC}"
./_output/test-agent -hub-address="localhost:${HUB_SERVER_GRPC_PORT}" -cluster-name="test-cluster" -insecure=true &
AGENT_PID=$!
sleep 3

# Check if agent is connected (this is harder to check directly, so we'll just wait)
echo -e "${GREEN}✓ test-agent started (PID: ${AGENT_PID})${NC}"
echo ""

echo -e "${GREEN}=== All services started successfully! ===${NC}"
echo ""
echo -e "${BLUE}Service Status:${NC}"
echo -e "  • test-simple-server: http://localhost:${SIMPLE_SERVER_PORT} (PID: ${SIMPLE_SERVER_PID})"
echo -e "  • test-server (hub):  http://localhost:${HUB_SERVER_HTTP_PORT} (PID: ${HUB_SERVER_PID})"
echo -e "  • test-agent:         Connected to localhost:${HUB_SERVER_GRPC_PORT} (PID: ${AGENT_PID})"
echo ""

echo -e "${YELLOW}=== Testing Instructions ===${NC}"
echo ""
echo -e "${BLUE}1. Test direct access to simple server:${NC}"
echo -e "   curl http://localhost:${SIMPLE_SERVER_PORT}/"
echo ""
echo -e "${BLUE}2. Test tunnel functionality (hub -> agent -> simple-server):${NC}"
echo -e "   curl http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/"
echo -e "   curl http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/api/health"
echo ""
echo -e "${BLUE}3. Test health endpoints:${NC}"
echo -e "   curl http://localhost:${SIMPLE_SERVER_PORT}/health"
echo -e "   curl http://localhost:${HUB_SERVER_HTTP_PORT}/health"
echo ""

echo -e "${YELLOW}Running basic connectivity tests...${NC}"
echo ""

# Test 1: Direct access to simple server
echo -e "${BLUE}Test 1: Direct access to simple server${NC}"
if curl -s http://localhost:${SIMPLE_SERVER_PORT}/ | grep -q "Hello World"; then
    echo -e "${GREEN}✓ Direct access works${NC}"
else
    echo -e "${RED}✗ Direct access failed${NC}"
fi
echo ""

# Test 2: Hub server health
echo -e "${BLUE}Test 2: Hub server health${NC}"
if curl -s http://localhost:${HUB_SERVER_HTTP_PORT}/health | grep -q "OK"; then
    echo -e "${GREEN}✓ Hub server health check works${NC}"
else
    echo -e "${RED}✗ Hub server health check failed${NC}"
fi
echo ""

# Test 3: Tunnel functionality
echo -e "${BLUE}Test 3: Tunnel functionality${NC}"
echo -e "Testing: curl http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/"
TUNNEL_RESPONSE=$(curl -s http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/ 2>/dev/null || echo "FAILED")
if echo "$TUNNEL_RESPONSE" | grep -q "Hello World"; then
    echo -e "${GREEN}✓ Tunnel functionality works!${NC}"
    echo -e "${GREEN}Response: ${TUNNEL_RESPONSE}${NC}"
else
    echo -e "${RED}✗ Tunnel functionality failed${NC}"
    echo -e "${RED}Response: ${TUNNEL_RESPONSE}${NC}"
fi
echo ""

echo -e "${YELLOW}=== Environment is ready for testing! ===${NC}"
echo -e "${YELLOW}Press Ctrl+C to stop all services${NC}"
echo ""

# Keep the script running until interrupted
while true; do
    sleep 1
done
