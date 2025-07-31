package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/onsi/ginkgo/v2"
	"github.com/xuezhaojun/multiclustertunnel/pkg/agent"
	"github.com/xuezhaojun/multiclustertunnel/pkg/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

// TestingInterface defines the interface for both testing.T and Ginkgo
type TestingInterface interface {
	Errorf(format string, args ...interface{})
	Logf(format string, args ...interface{})
}

// TestFramework provides a complete testing environment for integration tests
type TestFramework struct {
	t           TestingInterface
	ctx         context.Context
	cancel      context.CancelFunc
	hubServer   *server.Server
	agents      map[string]*agent.Agent
	mockServers map[string]*MockServer
	mu          sync.RWMutex

	// Configuration
	hubGRPCAddr   string
	hubHTTPAddr   string
	useTLS        bool
	grpcTLSConfig *tls.Config
	httpTLSConfig *tls.Config
}

// Note: The server now handles routing internally by parsing cluster names from URLs
// No need for a separate router implementation

// MockServer represents a mock backend server for testing
type MockServer struct {
	listener net.Listener
	server   *http.Server
	addr     string
	handler  http.HandlerFunc
	mu       sync.RWMutex
	requests []MockRequest
}

// MockRequest captures details of received requests
type MockRequest struct {
	Method    string
	Path      string
	Headers   http.Header
	Body      []byte
	Timestamp time.Time
}

// TestRequestProcessor implements agent.RequestProcessor for testing
type TestRequestProcessor struct{}

func (p *TestRequestProcessor) Process(targetServiceURL string, r *http.Request) (error, int) {
	// For testing, we don't need complex authentication logic
	// Just pass through all requests
	return nil, http.StatusOK
}

// TestCertificateProvider implements agent.CertificateProvider for testing
type TestCertificateProvider struct{}

func (c *TestCertificateProvider) GetRootCAs() (*x509.CertPool, error) {
	// For testing, return system root CAs or create an empty pool
	return x509.NewCertPool(), nil
}

// TestRouter implements agent.Router for testing
type TestRouter struct {
	targetAddr string
	mu         sync.RWMutex
}

func (r *TestRouter) ParseTargetService(req *http.Request) (targetproto, targethost, targetpath string, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.targetAddr == "" {
		return "", "", "", fmt.Errorf("no target address configured")
	}

	// For testing, we assume the target address is in the format "host:port"
	return "http", r.targetAddr, req.URL.Path, nil
}

func (r *TestRouter) SetTargetAddr(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.targetAddr = addr
}

type TestClusterNameParser struct{}

func (p *TestClusterNameParser) ParseClusterName(r *http.Request) (string, error) {
	return "test-cluster", nil
}

// TestServiceRouter implements agent.ServiceRouter for testing (legacy)
type TestServiceRouter struct {
	targetAddr   string
	mu           sync.RWMutex
	routes       map[string]string // service -> target_addr mapping for testing
	connRoutes   map[string]string // conn_id -> target_addr mapping for backward compatibility
	headerRoutes map[string]string // header_key:header_value -> target_addr mapping
}

func (r *TestServiceRouter) GetTargetAddress(headers map[string]string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Priority 1: Check header-based routing
	for key, value := range headers {
		routeKey := fmt.Sprintf("%s:%s", key, value)
		if addr, exists := r.headerRoutes[routeKey]; exists {
			return addr, nil
		}
	}

	// Priority 2: Check service-based routing from headers
	if service, exists := headers["service"]; exists {
		if addr, exists := r.routes[service]; exists {
			return addr, nil
		}
	}

	// Priority 3: Default to the configured target address
	if r.targetAddr == "" {
		return "", fmt.Errorf("no target address configured")
	}

	return r.targetAddr, nil
}

// SetRoute sets a route for a specific connection ID (backward compatibility)
func (r *TestServiceRouter) SetRoute(connID string, targetAddr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connRoutes == nil {
		r.connRoutes = make(map[string]string)
	}
	r.connRoutes[connID] = targetAddr
}

// SetServiceRoute sets a route for a specific service name
func (r *TestServiceRouter) SetServiceRoute(service string, targetAddr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.routes == nil {
		r.routes = make(map[string]string)
	}
	r.routes[service] = targetAddr
}

// SetHeaderRoute sets a route for a specific header key-value pair
func (r *TestServiceRouter) SetHeaderRoute(headerKey, headerValue, targetAddr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.headerRoutes == nil {
		r.headerRoutes = make(map[string]string)
	}
	routeKey := fmt.Sprintf("%s:%s", headerKey, headerValue)
	r.headerRoutes[routeKey] = targetAddr
}

func (r *TestServiceRouter) SetDefaultTarget(targetAddr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.targetAddr = targetAddr
}

// NewTestFramework creates a new test framework instance
func NewTestFramework(t TestingInterface, useTLS bool) *TestFramework {
	ctx, cancel := context.WithCancel(context.Background())

	framework := &TestFramework{
		t:           t,
		ctx:         ctx,
		cancel:      cancel,
		agents:      make(map[string]*agent.Agent),
		mockServers: make(map[string]*MockServer),
		useTLS:      useTLS,
		hubGRPCAddr: "localhost:0", // Use random port
		hubHTTPAddr: "localhost:0", // Use random port
	}

	if useTLS {
		framework.grpcTLSConfig = getTestTLSConfig()
		framework.httpTLSConfig = getTestTLSConfig()
	}

	return framework
}

// NewTestFrameworkWithTestingT creates a new test framework instance with testing.T
func NewTestFrameworkWithTestingT(t *testing.T, useTLS bool) *TestFramework {
	return NewTestFramework(t, useTLS)
}

// GinkgoTestingAdapter adapts Ginkgo's GinkgoTInterface to our TestingInterface
type GinkgoTestingAdapter struct {
	ginkgo.GinkgoTInterface
}

func (g *GinkgoTestingAdapter) Errorf(format string, args ...interface{}) {
	g.GinkgoTInterface.Errorf(format, args...)
}

func (g *GinkgoTestingAdapter) Logf(format string, args ...interface{}) {
	g.GinkgoTInterface.Logf(format, args...)
}

// NewTestFrameworkWithGinkgo creates a new test framework instance with Ginkgo
func NewTestFrameworkWithGinkgo(useTLS bool) *TestFramework {
	return NewTestFramework(&GinkgoTestingAdapter{ginkgo.GinkgoT()}, useTLS)
}

// Setup initializes the test environment
func (f *TestFramework) Setup() error {
	// Create and start the real Hub server
	if err := f.startHubServer(); err != nil {
		return fmt.Errorf("failed to start Hub server: %w", err)
	}

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Cleanup tears down the test environment
func (f *TestFramework) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Cancel context first to stop all agents from reconnecting
	f.cancel()

	// Give agents a moment to stop gracefully
	time.Sleep(100 * time.Millisecond)

	// Stop all agents gracefully
	for name, agent := range f.agents {
		klog.InfoS("Stopping agent", "name", name)
		// The agent should have stopped when context was cancelled
		if agent != nil {
			// No additional cleanup needed as agents use the framework context
		}
	}

	// Stop all mock servers
	for name, server := range f.mockServers {
		klog.InfoS("Stopping mock server", "name", name)
		server.Stop()
	}

	// Stop Hub server (this will stop both gRPC and HTTP servers)
	if f.hubServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		f.hubServer.Shutdown(ctx)
	}
}

// GetHubGRPCAddr returns the actual gRPC server address
func (f *TestFramework) GetHubGRPCAddr() string {
	// For now, we'll use the configured address
	// TODO: Get actual listening address from Hub server
	return f.hubGRPCAddr
}

// GetHubHTTPAddr returns the actual HTTP server address
func (f *TestFramework) GetHubHTTPAddr() string {
	// For now, we'll use the configured address
	// TODO: Get actual listening address from Hub server
	return f.hubHTTPAddr
}

// CreateMockServer creates a new mock backend server
func (f *TestFramework) CreateMockServer(name string, handler http.HandlerFunc) (*MockServer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	mockServer := &MockServer{
		listener: listener,
		addr:     listener.Addr().String(),
		handler:  handler,
		requests: make([]MockRequest, 0),
	}

	// Wrap handler to capture requests
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		mockServer.mu.Lock()
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))

		mockServer.requests = append(mockServer.requests, MockRequest{
			Method:    r.Method,
			Path:      r.URL.Path,
			Headers:   r.Header.Clone(),
			Body:      body,
			Timestamp: time.Now(),
		})
		mockServer.mu.Unlock()

		if handler != nil {
			handler(w, r)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}

	mockServer.server = &http.Server{
		Handler: http.HandlerFunc(wrappedHandler),
	}

	go func() {
		if err := mockServer.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			f.t.Errorf("Mock server %s failed: %v", name, err)
		}
	}()

	f.mockServers[name] = mockServer
	return mockServer, nil
}

// CreateAgent creates and starts a new agent client
func (f *TestFramework) CreateAgent(clusterName string, targetAddr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Note: The server now handles routing internally, no need to set cluster routes

	config := &agent.Config{
		HubAddress:  f.hubGRPCAddr,
		ClusterName: clusterName,
		BackoffFactory: func() backoff.BackOff {
			// Use a shorter backoff for tests to avoid hanging
			b := backoff.NewExponentialBackOff()
			b.InitialInterval = 100 * time.Millisecond
			b.MaxInterval = 1 * time.Second
			return b
		},
	}

	if f.useTLS {
		clientTLSConfig := getTestClientTLSConfig()
		config.DialOptions = append(config.DialOptions,
			grpc.WithTransportCredentials(credentials.NewTLS(clientTLSConfig)))
	} else {
		config.DialOptions = append(config.DialOptions,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Create test components for the agent
	requestProcessor := &TestRequestProcessor{}
	certProvider := &TestCertificateProvider{}
	router := &TestRouter{}
	router.SetTargetAddr(targetAddr)

	// Create the agent with the new architecture
	agentClient := agent.New(f.ctx, config, requestProcessor, certProvider, router)

	// Start the agent
	go func() {
		if err := agentClient.Run(f.ctx); err != nil {
			// Only log error if context is not cancelled (test not finished)
			if f.ctx.Err() == nil {
				f.t.Errorf("Agent %s failed: %v", clusterName, err)
			}
		}
	}()

	f.agents[clusterName] = agentClient
	return nil
}

// startHubServer starts the real Hub server
func (f *TestFramework) startHubServer() error {

	// Create hub server configuration with random ports
	config := &server.Config{
		GRPCListenAddress: "127.0.0.1:0", // Let the server pick a random port
		HTTPListenAddress: "127.0.0.1:0", // Let the server pick a random port
	}

	// Add TLS configuration if needed
	if f.useTLS {
		config.GRPCTLSConfig = f.grpcTLSConfig
		config.HTTPTLSConfig = f.httpTLSConfig
		klog.InfoS("Configuring Hub server with TLS")
	}

	// Create the hub server
	var err error
	f.hubServer, err = server.New(config, &TestClusterNameParser{})
	if err != nil {
		return fmt.Errorf("failed to create hub server: %w", err)
	}

	// Start the hub server in a goroutine
	go func() {
		if err := f.hubServer.Run(f.ctx); err != nil {
			if f.ctx.Err() == nil { // Only log if not cancelled
				f.t.Errorf("Hub server failed: %v", err)
			}
		}
	}()

	// Wait for server to be ready
	for i := 0; i < 50; i++ { // Wait up to 5 seconds
		if f.hubServer.Ready() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !f.hubServer.Ready() {
		return fmt.Errorf("hub server failed to become ready")
	}

	// Get the actual addresses after the server has started
	f.hubGRPCAddr = f.hubServer.GRPCAddress()
	f.hubHTTPAddr = f.hubServer.HTTPAddress()

	return nil
}

// Stop stops the mock server
func (m *MockServer) Stop() {
	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.server.Shutdown(ctx)
	}
	if m.listener != nil {
		m.listener.Close()
	}
}

// GetAddr returns the server address
func (m *MockServer) GetAddr() string {
	return m.addr
}

// GetRequests returns all captured requests
func (m *MockServer) GetRequests() []MockRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requests := make([]MockRequest, len(m.requests))
	copy(requests, m.requests)
	return requests
}

// ClearRequests clears all captured requests
func (m *MockServer) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
}

// GetGRPCListener creates a new gRPC listener for custom testing
func (f *TestFramework) GetGRPCListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}
