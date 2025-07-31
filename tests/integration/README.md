# Integration Tests

This directory contains comprehensive integration tests for the multiclustertunnel project. These tests verify the end-to-end functionality of the tunnel system including hub-agent communication, HTTP request forwarding, error handling, and reconnection scenarios.

## Overview

The integration tests are designed to:

1. **Test Basic Functionality**: Verify basic tunnel connectivity, concurrent requests, large data transfers, and different HTTP methods
2. **Test Error Scenarios**: Validate error handling for connection failures, timeouts, DNS resolution failures, and invalid requests
3. **Test Agent Reconnection**: Ensure agents can properly reconnect after network interruptions and handle backoff strategies
4. **Test TLS Security**: Verify secure communication using embedded test certificates

## Test Structure

### Core Components

- **`framework.go`**: Main testing framework that provides a complete test environment
- **`certs.go`**: Embedded test certificates for TLS testing (DO NOT use in production)
- **`basic_test.go`**: Basic functionality tests
- **`error_test.go`**: Error scenario tests
- **`reconnect_test.go`**: Agent reconnection and resilience tests
- **`drain_test.go`**: DRAIN signal integration tests
- **`integration_suite_test.go`**: Ginkgo test suite configuration

### Test Framework Features

The `TestFramework` provides:

- **Hub Server**: Complete gRPC and HTTP server setup
- **Mock Backend Servers**: Configurable HTTP servers for testing
- **Agent Management**: Automatic agent creation and lifecycle management
- **TLS Support**: Built-in TLS configuration with test certificates
- **Request Tracking**: Capture and verify backend requests
- **Resource Cleanup**: Automatic cleanup of all test resources

## Running Tests

### Prerequisites

- Go 1.22+
- All project dependencies installed (`go mod tidy`)
- Ginkgo testing framework (automatically installed with dependencies)

### Commands

```bash
# Run all integration tests
make test-integration

# Run specific test
go test -v ./tests/integration -run TestBasicConnectivity

# Run tests without race detector (faster)
go test -v ./tests/integration

# Run all tests (unit + integration + e2e)
make test-all
```

### Ginkgo Testing Framework

Tests use the Ginkgo BDD testing framework. You can run tests using:
- `ginkgo ./tests/integration` - Use Ginkgo runner
- `go test ./tests/integration` - Use standard Go test

Ginkgo provides enhanced test output, parallel execution, and better test organization.

### Test Categories

#### Basic Functionality Tests
- `TestBasicConnectivity`: Basic tunnel connectivity
- `TestConcurrentRequests`: Multiple concurrent requests handling
- `TestLargeDataTransfer`: Large file transfer (1MB)
- `TestHTTPMethods`: Different HTTP methods (GET, POST, PUT, DELETE, PATCH)
- `TestTLSConnectivity`: TLS-enabled communication

#### Error Scenario Tests
- `TestClusterNotFound`: Requests to non-existent clusters
- `TestBackendConnectionRefused`: Backend service unavailable
- `TestRequestTimeout`: Request timeout handling
- `TestClientCancellation`: Client request cancellation
- `TestInvalidClusterName`: Invalid cluster name handling
- `TestBackendSlowResponse`: Slow backend response handling
- `TestBackendError`: Backend error propagation
- `TestDNSResolutionFailure`: DNS resolution failures

#### Reconnection Tests
- `TestAgentReconnection`: Agent reconnection after hub restart
- `TestAgentReconnectionWithBackoff`: Backoff strategy verification
- `TestMultipleAgentReconnection`: Multiple agents reconnecting
- `TestAgentGracefulShutdown`: Graceful shutdown handling
- `TestConnectionPoolManagement`: Connection pool behavior
- `TestHeartbeatMechanism`: Keepalive mechanism
- `TestConcurrentReconnections`: Multiple agents reconnecting simultaneously

#### DRAIN Signal Tests
- `TestDRAINPacketHandling`: DRAIN signal processing during graceful shutdown
- `TestMultipleAgentsDRAIN`: Multiple agents sending DRAIN signals simultaneously

## Test Certificates

The integration tests use embedded test certificates for TLS testing:

- **CA Certificate**: Self-signed root CA
- **Server Certificate**: Server certificate signed by the test CA
- **Client Configuration**: Automatic client TLS configuration

**⚠️ WARNING**: These certificates are for testing only and should NEVER be used in production environments.

## Configuration

### Test Framework Options

```go
// Create framework without TLS
framework := NewTestFramework(t, false)

// Create framework with TLS
framework := NewTestFramework(t, true)
```

### Mock Server Configuration

```go
// Create a simple mock server
mockServer, err := framework.CreateMockServer("backend", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Hello from backend"))
})

// Create an agent that routes to the mock server
err = framework.CreateAgent("test-cluster", mockServer.GetAddr())
```

## Known Issues

1. **Data Race Warnings**: The current codebase has some data race conditions that are detected when running with `-race`. These are existing issues in the main codebase, not in the test framework.

2. **Test Timeouts**: Some tests may timeout if the system is under heavy load. The default timeout is 30 seconds for most operations.

## Troubleshooting

### Test Failures

1. **Connection Timeouts**: Increase wait times in tests or check system resources
2. **Port Conflicts**: Tests use random ports, but conflicts can still occur
3. **Race Conditions**: Run without `-race` flag if encountering race condition warnings from the main codebase

### Debugging

Enable verbose logging:
```bash
go test -v ./tests/integration -args -v=4
```

### Common Issues

- **"Request timeout"**: Usually indicates communication issues between hub and agent
- **"Cluster not available"**: Agent hasn't connected or has disconnected
- **"Connection refused"**: Backend service is not running or unreachable

## Contributing

When adding new integration tests:

1. Use the existing `TestFramework` for consistency
2. Follow the naming convention: `Test[Feature][Scenario]`
3. Include proper cleanup with `defer framework.Cleanup()`
4. Add appropriate assertions and error checking
5. Document any special requirements or configurations

## Future Improvements

- [ ] Add performance benchmarks
- [ ] Implement chaos testing scenarios
- [ ] Add metrics collection and verification
- [ ] Improve test isolation and parallelization
- [ ] Add integration with CI/CD pipelines
