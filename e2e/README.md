# End-to-End Testing for MultiClusterTunnel

This directory contains end-to-end tests for the multiclustertunnel project using Kind clusters and the Kubernetes e2e-framework.

## Overview

The e2e tests validate the complete functionality of the multiclustertunnel system by:

1. **Kind Cluster Setup**: Creating a multi-node Kubernetes cluster using Kind
2. **Multi-Namespace Deployment**: Deploying server (hub) and agent components in separate namespaces
3. **Certificate Management**: Generating and distributing TLS certificates for secure gRPC communication
4. **Service Connectivity**: Verifying agent-to-server connectivity through Kubernetes services
5. **Functional Testing**: Testing tunnel establishment, HTTP routing, and data transfer

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kind Cluster                             │
│                                                             │
│  ┌─────────────────┐         ┌─────────────────────────────┐│
│  │ mctunnel-hub    │         │ mctunnel-agent              ││
│  │ namespace       │         │ namespace                   ││
│  │                 │         │                             ││
│  │ ┌─────────────┐ │         │ ┌─────────────┐             ││
│  │ │   Server    │ │◄────────┤ │   Agent     │             ││
│  │ │ (Hub Core)  │ │  gRPC   │ │ (Agent Core)│             ││
│  │ │             │ │  :8443  │ │             │             ││
│  │ └─────────────┘ │         │ └─────────────┘             ││
│  │                 │         │                             ││
│  │ ┌─────────────┐ │         │ ┌─────────────┐             ││
│  │ │ HTTP Server │ │         │ │ Mock Backend│             ││
│  │ │    :8080    │ │         │ │  Services   │             ││
│  │ └─────────────┘ │         │ └─────────────┘             ││
│  └─────────────────┘         └─────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
e2e/
├── main_test.go                    # Test entry point and environment setup
├── basic_connectivity_test.go      # Basic tunnel connectivity tests
├── certificate_test.go             # Certificate validation tests
├── multi_namespace_test.go         # Multi-namespace communication tests
├── templates/                      # Kubernetes resource templates
│   ├── kind.config                # Kind cluster configuration
│   ├── namespaces/                # Namespace templates
│   ├── certificates/              # Certificate secret templates
│   ├── server/                    # Server deployment templates
│   ├── agent/                     # Agent deployment templates
│   └── mock-services/             # Mock backend service templates
├── utils/                         # Utility functions
│   ├── certificates.go           # Certificate generation utilities
│   ├── templates.go              # Template rendering functions
│   └── cluster.go                # Cluster management utilities
└── README.md                     # This file
```

## Prerequisites

- Go 1.24+
- Docker
- Kind v0.20.0+
- kubectl

## Running Tests

### Local Development

```bash
# Build required images
make build-e2e-images

# Run all e2e tests
make test-e2e

# Run specific test
go test -v ./e2e -run TestBasicConnectivity
```

### CI/CD

The tests are automatically run in GitHub Actions on:
- Pull requests
- Pushes to main branch
- Multiple Kubernetes versions (1.28, 1.29, 1.30)

## Test Categories

### 1. Basic Connectivity Tests
- Server and agent deployment
- gRPC tunnel establishment
- Basic HTTP request routing

### 2. Certificate Tests
- TLS certificate validation
- Secure gRPC communication
- Certificate rotation scenarios

### 3. Multi-Namespace Tests
- Cross-namespace service communication
- RBAC validation
- Network policy compliance

## Configuration

### Environment Variables

- `SERVER_IMAGE`: Docker image for the server component
- `AGENT_IMAGE`: Docker image for the agent component
- `KIND_IMAGE`: Kind node image (default: kindest/node:v1.30.2)

### Command Line Flags

- `-server-image`: Server docker image
- `-agent-image`: Agent docker image
- `-kind-image`: Kind node image

## Troubleshooting

### Common Issues

1. **Kind cluster creation fails**
   - Check Docker daemon is running
   - Ensure sufficient disk space
   - Verify Kind version compatibility

2. **Image loading fails**
   - Verify images are built correctly
   - Check image names match configuration
   - Ensure Kind cluster is running

3. **Certificate issues**
   - Check certificate generation logs
   - Verify secret mounting in pods
   - Validate certificate SAN entries

4. **Connectivity issues**
   - Check service DNS resolution
   - Verify network policies
   - Examine pod logs for errors

### Debug Commands

```bash
# Check cluster status
kubectl get nodes
kubectl get pods -A

# Check certificates
kubectl get secrets -n mctunnel-hub
kubectl get secrets -n mctunnel-agent

# Check logs
kubectl logs -n mctunnel-hub deployment/mctunnel-server
kubectl logs -n mctunnel-agent deployment/mctunnel-agent

# Port forward for debugging
kubectl port-forward -n mctunnel-hub svc/mctunnel-server 8080:8080
```

## Contributing

When adding new tests:

1. Follow the existing test structure
2. Use the e2e-framework patterns
3. Ensure proper cleanup in test teardown
4. Add appropriate documentation
5. Test both success and failure scenarios

## References

- [sigs.k8s.io/e2e-framework](https://github.com/kubernetes-sigs/e2e-framework)
- [Kind Documentation](https://kind.sigs.k8s.io/)
- [MultiClusterTunnel Architecture](../docs/)
