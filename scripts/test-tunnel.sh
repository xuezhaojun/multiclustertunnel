#!/bin/bash

# Script to test the multiclustertunnel setup
# This script assumes all services are already running

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SIMPLE_SERVER_PORT=9090
HUB_SERVER_HTTP_PORT=8080

echo -e "${BLUE}=== MultiCluster Tunnel Test Suite ===${NC}"
echo ""

# Test 1: Direct access to simple server
echo -e "${BLUE}Test 1: Direct access to test-simple-server${NC}"
echo -e "Command: curl http://localhost:${SIMPLE_SERVER_PORT}/"
echo ""
RESPONSE=$(curl -s http://localhost:${SIMPLE_SERVER_PORT}/ 2>/dev/null || echo "FAILED")
if echo "$RESPONSE" | grep -q "Hello World"; then
    echo -e "${GREEN}✓ PASS: Direct access works${NC}"
    echo -e "Response: $RESPONSE"
else
    echo -e "${RED}✗ FAIL: Direct access failed${NC}"
    echo -e "Response: $RESPONSE"
fi
echo ""
echo "---"
echo ""

# Test 2: Hub server health check
echo -e "${BLUE}Test 2: Hub server health check${NC}"
echo -e "Command: curl http://localhost:${HUB_SERVER_HTTP_PORT}/health"
echo ""
RESPONSE=$(curl -s http://localhost:${HUB_SERVER_HTTP_PORT}/health 2>/dev/null || echo "FAILED")
if echo "$RESPONSE" | grep -q "OK"; then
    echo -e "${GREEN}✓ PASS: Hub server health check works${NC}"
    echo -e "Response: $RESPONSE"
else
    echo -e "${RED}✗ FAIL: Hub server health check failed${NC}"
    echo -e "Response: $RESPONSE"
fi
echo ""
echo "---"
echo ""

# Test 3: Tunnel functionality - Root path
echo -e "${BLUE}Test 3: Tunnel functionality (root path)${NC}"
echo -e "Command: curl http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/"
echo ""
RESPONSE=$(curl -s http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/ 2>/dev/null || echo "FAILED")
if echo "$RESPONSE" | grep -q "Hello World"; then
    echo -e "${GREEN}✓ PASS: Tunnel works for root path${NC}"
    echo -e "Response: $RESPONSE"
else
    echo -e "${RED}✗ FAIL: Tunnel failed for root path${NC}"
    echo -e "Response: $RESPONSE"
fi
echo ""
echo "---"
echo ""

# Test 4: Tunnel functionality - API path
echo -e "${BLUE}Test 4: Tunnel functionality (API path)${NC}"
echo -e "Command: curl http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/api/v1/pods"
echo ""
RESPONSE=$(curl -s http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/api/v1/pods 2>/dev/null || echo "FAILED")
if echo "$RESPONSE" | grep -q "test-simple-server"; then
    echo -e "${GREEN}✓ PASS: Tunnel works for API path${NC}"
    echo -e "Response: $RESPONSE"
else
    echo -e "${RED}✗ FAIL: Tunnel failed for API path${NC}"
    echo -e "Response: $RESPONSE"
fi
echo ""
echo "---"
echo ""

# Test 5: Tunnel functionality - Health endpoint through tunnel
echo -e "${BLUE}Test 5: Tunnel functionality (health endpoint)${NC}"
echo -e "Command: curl http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/health"
echo ""
RESPONSE=$(curl -s http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/health 2>/dev/null || echo "FAILED")
if echo "$RESPONSE" | grep -q "OK"; then
    echo -e "${GREEN}✓ PASS: Tunnel works for health endpoint${NC}"
    echo -e "Response: $RESPONSE"
else
    echo -e "${RED}✗ FAIL: Tunnel failed for health endpoint${NC}"
    echo -e "Response: $RESPONSE"
fi
echo ""
echo "---"
echo ""

# Test 6: JSON API endpoint through tunnel
echo -e "${BLUE}Test 6: Tunnel functionality (JSON API endpoint)${NC}"
echo -e "Command: curl http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/api/test"
echo ""
RESPONSE=$(curl -s http://localhost:${HUB_SERVER_HTTP_PORT}/test-cluster/api/test 2>/dev/null || echo "FAILED")
if echo "$RESPONSE" | grep -q "test-simple-server"; then
    echo -e "${GREEN}✓ PASS: Tunnel works for JSON API endpoint${NC}"
    echo -e "Response: $RESPONSE"
else
    echo -e "${RED}✗ FAIL: Tunnel failed for JSON API endpoint${NC}"
    echo -e "Response: $RESPONSE"
fi
echo ""

echo -e "${BLUE}=== Test Summary ===${NC}"
echo ""
echo -e "${YELLOW}If all tests pass, your multiclustertunnel setup is working correctly!${NC}"
echo ""
echo -e "${BLUE}Architecture:${NC}"
echo -e "  curl -> test-server (hub) -> test-agent (tunnel) -> test-simple-server"
echo ""
echo -e "${BLUE}Flow:${NC}"
echo -e "  1. HTTP request sent to hub server (port ${HUB_SERVER_HTTP_PORT})"
echo -e "  2. Hub server forwards request through gRPC tunnel to agent"
echo -e "  3. Agent receives request and proxies it to test-simple-server (port ${SIMPLE_SERVER_PORT})"
echo -e "  4. Response flows back through the same path"
echo ""
