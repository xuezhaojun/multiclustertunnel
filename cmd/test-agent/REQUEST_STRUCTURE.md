# Test Agent Request Structure Documentation

This document explains the request structure for testing the multiclustertunnel agent with the test implementations.

## Overview

The test agent implements three core interfaces with simple, clear logic suitable for testing:

1. **TestRequestProcessor** - Handles HTTP request processing and authentication
2. **TestCertificateProvider** - Provides root CAs for TLS connections  
3. **TestRouter** - Parses requests to determine target service URLs

## Packet Structure

All communication between the hub and agent happens through gRPC streaming with `Packet` messages:

```protobuf
message Packet {
  int64 conn_id = 1;           // Unique connection ID for multiplexing
  ControlCode code = 2;        // DATA(0), ERROR(1), or DRAIN(2)
  bytes data = 3;              // HTTP request data (raw HTTP format)
  string error_message = 4;    // Only used when code = ERROR
}
```

## HTTP Request Data Format

The `data` field in DATA packets contains raw HTTP/1.1 format requests:

```http
GET /test-cluster/api/v1/pods HTTP/1.1\r\n
Host: kubernetes.default.svc\r\n
Authorization: Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6...\r\n
User-Agent: kubectl/v1.28.0\r\n
Accept: application/json\r\n
\r\n
```

## URL Routing Patterns

### 1. Kube-apiserver Requests
**Pattern**: `/<cluster-name>/api/...`

**Examples**:
- `GET /test-cluster/api/v1/pods`
- `GET /test-cluster/api/v1/namespaces/default/pods`
- `POST /test-cluster/api/v1/namespaces/default/pods`

**Target**: Routes to `kubernetes.default.svc`

### 2. Service Proxy Requests  
**Pattern**: `/<cluster-name>/api/v1/namespaces/<namespace>/services/<service>/proxy-service/<path>`

**Examples**:
- `GET /test-cluster/api/v1/namespaces/default/services/https:my-service:443/proxy-service/health`
- `POST /test-cluster/api/v1/namespaces/monitoring/services/https:prometheus:9090/proxy-service/api/v1/query`

**Target**: Routes to `<service>.<namespace>.svc:<port>`

## Test Implementation Behavior

### TestRequestProcessor
- **Authentication**: Accepts all requests (logs auth header if present)
- **Authorization**: Always returns HTTP 200 OK
- **Processing**: Logs request details for debugging

### TestCertificateProvider  
- **Root CAs**: Uses system certificate pool when available
- **Fallback**: Creates empty pool if system CAs unavailable
- **TLS**: Enables secure connections to target services

### TestRouter
- **Service Detection**: Checks for "proxy-service" in URL path
- **Service Routing**: Routes to `test-service.default.svc:443` (simplified)
- **API Routing**: Routes to `kubernetes.default.svc` for all other requests
- **Logging**: Logs routing decisions for debugging

## Testing Examples

### Example 1: List Pods
```bash
# Request to hub server
curl -H "Authorization: Bearer <token>" \
     https://hub-server/test-cluster/api/v1/pods

# Packet sent to agent
{
  "conn_id": 12345,
  "code": "DATA", 
  "data": "GET /test-cluster/api/v1/pods HTTP/1.1\r\nHost: kubernetes.default.svc\r\n..."
}

# TestRouter routes to: https://kubernetes.default.svc/api/v1/pods
```

### Example 2: Service Health Check
```bash
# Request to hub server  
curl -H "Authorization: Bearer <token>" \
     https://hub-server/test-cluster/api/v1/namespaces/default/services/https:my-app:8080/proxy-service/health

# Packet sent to agent
{
  "conn_id": 12346,
  "code": "DATA",
  "data": "GET /test-cluster/api/v1/namespaces/default/services/https:my-app:8080/proxy-service/health HTTP/1.1\r\n..."
}

# TestRouter routes to: https://test-service.default.svc:443/health
```

## Connection Lifecycle

1. **Establishment**: Hub sends initial empty packet with target address
2. **HTTP Request**: Hub sends HTTP request as DATA packet  
3. **Data Flow**: Bidirectional data packets with same conn_id
4. **Cleanup**: Connection closed when HTTP response complete

## Error Handling

- **Authentication Errors**: TestRequestProcessor returns HTTP 401/403
- **Routing Errors**: TestRouter returns error for malformed URLs
- **Connection Errors**: Agent sends ERROR packet with error_message
- **TLS Errors**: TestCertificateProvider handles certificate validation

## Debugging

Enable verbose logging to see detailed request processing:

```bash
./test-agent -v=4 -hub-address=localhost:8443 -cluster-name=test-cluster
```

This will log:
- Request processing details
- Certificate loading status  
- Routing decisions
- Target service URLs
