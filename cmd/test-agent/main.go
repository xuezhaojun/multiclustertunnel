// Test Agent Client with Simple Interface Implementations
//
// This program demonstrates how to implement the three core interfaces required by the agent:
// 1. RequestProcessor - handles HTTP request processing and authentication
// 2. CertificateProvider - provides root CAs for TLS connections
// 3. Router - parses requests to determine target service URLs
//
// REQUEST STRUCTURE FOR TESTING:
//
// The server expects HTTP requests to be sent through the tunnel as packets with the following structure:
//
// Packet Structure (protobuf):
//
//	{
//	  "conn_id": <int64>,           // Unique connection ID for multiplexing
//	  "code": "DATA",               // ControlCode: DATA(0), ERROR(1), or DRAIN(2)
//	  "data": <bytes>,              // HTTP request data (raw HTTP format)
//	  "error_message": <string>     // Only used when code = ERROR
//	}
//
// HTTP Request Data Format (in packet.data field):
// The data field contains the raw HTTP request in standard HTTP/1.1 format:
//
// "GET /cluster-name/api/v1/pods HTTP/1.1\r\n
//
//	Host: kubernetes.default.svc\r\n
//	Authorization: Bearer <token>\r\n
//	User-Agent: kubectl/v1.28.0\r\n
//	\r\n
//	<request body if any>"
//
// URL Patterns for Testing:
// 1. Kube-apiserver requests: /<cluster-name>/api/v1/pods
// 2. Service proxy requests: /<cluster-name>/api/v1/namespaces/<ns>/services/<service>/proxy-service/<path>
//
// Example Test Requests:
// - GET /test-cluster/api/v1/pods (routes to kubernetes.default.svc)
// - GET /test-cluster/api/v1/namespaces/default/services/https:my-service:443/proxy-service/health (routes to my-service.default.svc:443)
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/xuezhaojun/multiclustertunnel/pkg/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

// TestRequestProcessor implements agent.RequestProcessor for testing
// It provides simple authentication logic for testing purposes
//
// In production, this would:
// - Validate authentication tokens
// - Handle user impersonation for hub users
// - Process authorization headers
// - Return appropriate HTTP status codes for failures
type TestRequestProcessor struct{}

func (p *TestRequestProcessor) Process(targetServiceURL string, r *http.Request) (error, int) {
	// Simple test logic: allow all requests to pass through
	// In a real implementation, this would handle authentication, authorization, etc.
	klog.V(4).InfoS("Processing request", "target", targetServiceURL, "method", r.Method, "path", r.URL.Path)

	// For testing, we can add some basic validation
	if r.Header.Get("Authorization") == "" {
		klog.V(4).InfoS("No authorization header found, but allowing for testing")
	} else {
		klog.V(4).InfoS("Found authorization header", "auth_type", strings.Split(r.Header.Get("Authorization"), " ")[0])
	}

	// Always return success for testing
	return nil, http.StatusOK
}

// TestCertificateProvider implements agent.CertificateProvider for testing
// It provides a simple certificate pool for testing purposes
//
// In production, this would:
// - Load Kubernetes service account CA certificate from /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
// - Load additional CA certificates for OpenShift services
// - Provide proper root CAs for validating target service certificates
type TestCertificateProvider struct{}

func (c *TestCertificateProvider) GetRootCAs() (*x509.CertPool, error) {
	// For testing, create a basic certificate pool
	// In a real implementation, this would load actual CA certificates
	rootCAs := x509.NewCertPool()

	// Try to load system root CAs for better compatibility
	if systemRoots, err := x509.SystemCertPool(); err == nil {
		rootCAs = systemRoots
		klog.V(4).InfoS("Loaded system root CAs for testing")
	} else {
		klog.V(4).InfoS("Could not load system root CAs, using empty pool", "error", err)
	}

	return rootCAs, nil
}

// TestRouter implements agent.Router for testing
// It provides simple routing logic for testing purposes
//
// In production, this would:
// - Parse complex URL patterns to identify target service type (kube-apiserver vs service)
// - Extract namespace, service name, and port information for service requests
// - Construct proper target URLs for HTTPS connections
// - Support both kube-apiserver and service proxy patterns
type TestRouter struct{}

func (r *TestRouter) ParseTargetService(req *http.Request) (string, string, string, error) {
	// Test routing logic that handles two different target scenarios
	// This mimics the real RouterImpl but with simplified logic for testing

	klog.V(4).InfoS("Routing request", "method", req.Method, "path", req.URL.Path, "uri", req.RequestURI)

	// Parse the request URI to extract routing information
	pathParams := strings.Split(req.URL.Path, "/")

	// Extract the target identifier from the first path segment
	targetIdentifier := pathParams[1]

	// Route based on the target identifier
	switch targetIdentifier {
	case "test-simpler-server":
		// Route to test-simple-server running on localhost:9090
		klog.V(4).InfoS("Routing to test-simple-server", "target", "localhost:9090")
		targetPath := "/" + strings.Join(pathParams[2:], "/") // Remove the target identifier from path
		return "http", "localhost:9090", targetPath, nil

	case "test-kind-cluster":
		// Route to Kubernetes API server in kind cluster (typically localhost:8081 or similar)
		// For testing purposes, we'll use a common kind cluster API server address
		klog.V(4).InfoS("Routing to test-kind-cluster", "target", "localhost:8081")
		targetPath := "/" + strings.Join(pathParams[2:], "/") // Remove the target identifier from path
		return "http", "localhost:8081", targetPath, nil

	default:
		return "", "", "", fmt.Errorf("unknown target identifier: %s, expected 'test-simpler-server' or 'test-kind-cluster'", targetIdentifier)
	}
}

func main() {
	var (
		hubAddress    = flag.String("hub-address", "localhost:8443", "Address of the hub server")
		clusterName   = flag.String("cluster-name", "test-cluster", "Name of the cluster")
		socketPath    = flag.String("socket-path", "/tmp/multiclustertunnel.sock", "Path for Unix Domain Socket")
		useInsecure   = flag.Bool("insecure", false, "Use insecure connection (no TLS)")
		skipTLSVerify = flag.Bool("skip-tls-verify", false, "Skip TLS certificate verification (for testing)")
	)
	klog.InitFlags(nil)
	flag.Parse()
	klog.InfoS("Starting test-agent-client",
		"hubAddress", *hubAddress,
		"clusterName", *clusterName,
		"socketPath", *socketPath,
		"insecure", *useInsecure)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create agent configuration
	config := &agent.Config{
		HubAddress:    *hubAddress,
		ClusterName:   *clusterName,
		UDSSocketPath: *socketPath,
	}

	// Configure TLS options
	if *useInsecure {
		// Use insecure connection (no TLS)
		config.DialOptions = append(config.DialOptions,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		klog.InfoS("Using insecure connection (no TLS)")
	} else if *skipTLSVerify {
		// Use TLS but skip certificate verification (for testing)
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		config.DialOptions = append(config.DialOptions,
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		klog.InfoS("Using TLS with certificate verification disabled (testing only)")
	} else {
		// Use TLS with proper certificate verification (default)
		tlsConfig := &tls.Config{}
		config.DialOptions = append(config.DialOptions,
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		klog.InfoS("Using TLS with certificate verification enabled")
	}

	// Create the agent client with test implementations
	client := agent.New(ctx, config,
		&TestRequestProcessor{},
		&TestCertificateProvider{},
		&TestRouter{},
	)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		klog.InfoS("Received shutdown signal, stopping agent")
		cancel()
	}()

	// Run the agent client
	klog.InfoS("Agent client starting...")
	if err := client.Run(ctx); err != nil && err != context.Canceled {
		klog.ErrorS(err, "Agent client failed")
		os.Exit(1)
	}

	klog.InfoS("Agent client stopped")
}
