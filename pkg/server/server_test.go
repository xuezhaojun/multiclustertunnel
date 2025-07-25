package server

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	v1 "github.com/xuezhaojun/multiclustertunnel/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func TestNewServer(t *testing.T) {
	// Test creating a server with default config
	server, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create server with default config: %v", err)
	}
	if server == nil {
		t.Fatal("Server should not be nil")
	}
}

func TestNewServerWithCustomConfig(t *testing.T) {
	// Test creating a server with custom config
	config := &Config{
		GRPCListenAddress: ":9999",
		ServerOptions: []grpc.ServerOption{
			grpc.Creds(insecure.NewCredentials()),
		},
		KeepAliveParams: &keepalive.ServerParameters{
			MaxConnectionIdle:     10 * time.Second,
			MaxConnectionAge:      20 * time.Second,
			MaxConnectionAgeGrace: 3 * time.Second,
			Time:                  3 * time.Second,
			Timeout:               500 * time.Millisecond,
		},
	}

	server, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create server with custom config: %v", err)
	}
	if server == nil {
		t.Fatal("Server should not be nil")
	}
}

func TestServerLifecycle(t *testing.T) {
	// Test server lifecycle (start and shutdown)
	server, err := New(&Config{
		GRPCListenAddress: ":0", // Use random available port
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test that server is not ready initially
	if server.Ready() {
		t.Error("Server should not be ready before starting")
	}

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx)
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Test that server is ready after starting
	if !server.Ready() {
		t.Error("Server should be ready after starting")
	}

	// Test graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}

	// Wait for Run to complete
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Errorf("Server Run returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server Run did not complete within timeout")
	}

	// Test that server is not ready after shutdown
	if server.Ready() {
		t.Error("Server should not be ready after shutdown")
	}
}

func TestNewStreamWithoutConnection(t *testing.T) {
	// Test creating a stream when no connection exists
	server, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()

	// Get connection for nonexistent cluster
	conn := server.GetTunnel("nonexistent-cluster")
	if conn != nil {
		t.Error("Expected nil connection for nonexistent cluster")
		return
	}

	// This should be nil since no connection exists
	if conn != nil {
		_, err := conn.NewPacketConn(ctx)
		if err == nil {
			t.Error("Expected error when creating stream for nonexistent cluster")
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	// Test default configuration
	config := DefaultConfig()
	if config == nil {
		t.Fatal("DefaultConfig should not return nil")
	}

	if config.GRPCListenAddress != ":8443" {
		t.Errorf("Expected default gRPC listen address :8443, got %s", config.GRPCListenAddress)
	}

	if config.HTTPListenAddress != ":8080" {
		t.Errorf("Expected default HTTP listen address :8080, got %s", config.HTTPListenAddress)
	}

	if config.KeepAliveParams == nil {
		t.Error("Default config should have KeepAliveParams")
	}
}

func TestServerWithTLSConfig(t *testing.T) {
	// Create test TLS configurations
	grpcTLSConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	httpTLSConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	// Test creating a server with separate TLS configs for gRPC and HTTP
	config := &Config{
		GRPCListenAddress: ":9999",
		HTTPListenAddress: ":8080",
		Router:            &TestRouter{}, // Simple test router
		GRPCTLSConfig:     grpcTLSConfig,
		HTTPTLSConfig:     httpTLSConfig,
	}

	server, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create server with TLS config: %v", err)
	}
	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Verify that HTTP server has TLS config set
	if server.httpServer == nil {
		t.Fatal("HTTP server should not be nil when router is provided")
	}
	if server.httpServer.TLSConfig == nil {
		t.Error("HTTP server should have TLS config when HTTPTLSConfig is enabled")
	}
	if server.httpServer.TLSConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected HTTP TLS MinVersion %d, got %d", tls.VersionTLS13, server.httpServer.TLSConfig.MinVersion)
	}
}

func TestServerWithOnlyGRPCTLS(t *testing.T) {
	// Create test TLS configuration only for gRPC
	grpcTLSConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	config := &Config{
		GRPCListenAddress: ":9999",
		HTTPListenAddress: ":8080",
		Router:            &TestRouter{},
		GRPCTLSConfig:     grpcTLSConfig,
		HTTPTLSConfig:     nil, // No TLS for HTTP
	}

	server, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create server with gRPC TLS config: %v", err)
	}
	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Verify that HTTP server does NOT have TLS config set
	if server.httpServer == nil {
		t.Fatal("HTTP server should not be nil when router is provided")
	}
	if server.httpServer.TLSConfig != nil {
		t.Error("HTTP server should not have TLS config when HTTPTLSConfig is nil")
	}
}

func TestServerWithOnlyHTTPTLS(t *testing.T) {
	// Create test TLS configuration only for HTTP
	httpTLSConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	config := &Config{
		GRPCListenAddress: ":9999",
		HTTPListenAddress: ":8080",
		Router:            &TestRouter{},
		GRPCTLSConfig:     nil, // No TLS for gRPC
		HTTPTLSConfig:     httpTLSConfig,
	}

	server, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create server with HTTP TLS config: %v", err)
	}
	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Verify that HTTP server has TLS config set
	if server.httpServer == nil {
		t.Fatal("HTTP server should not be nil when router is provided")
	}
	if server.httpServer.TLSConfig == nil {
		t.Error("HTTP server should have TLS config when HTTPTLSConfig is enabled")
	}
	if server.httpServer.TLSConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected HTTP TLS MinVersion %d, got %d", tls.VersionTLS13, server.httpServer.TLSConfig.MinVersion)
	}
}

// TestRouter is a simple router implementation for testing
type TestRouter struct{}

func (r *TestRouter) Parse(req *http.Request) (string, string, error) {
	return "test-cluster", "localhost:8080", nil
}

// testPacketConnection is a test implementation that captures sent data
type testPacketConnection struct {
	id          int64
	sentPackets []*v1.Packet
}

func newTestPacketConnection() *testPacketConnection {
	return &testPacketConnection{
		id:          123,
		sentPackets: make([]*v1.Packet, 0),
	}
}

func (t *testPacketConnection) ID() int64 {
	return t.id
}

func (t *testPacketConnection) Send(packet *v1.Packet) error {
	// Store the sent packet for verification
	t.sentPackets = append(t.sentPackets, packet)
	return nil
}

func (t *testPacketConnection) Recv() <-chan *v1.Packet {
	return make(chan *v1.Packet)
}

func (t *testPacketConnection) Close(err error) {
	// Not needed for this test
}

func (t *testPacketConnection) Context() context.Context {
	return context.Background()
}

func TestSendInitialHTTPRequestPreservesProtocolVersion(t *testing.T) {
	tests := []struct {
		name         string
		protoMajor   int
		protoMinor   int
		expectedLine string
	}{
		{
			name:         "HTTP/1.0",
			protoMajor:   1,
			protoMinor:   0,
			expectedLine: "GET /test HTTP/1.0\r\n",
		},
		{
			name:         "HTTP/1.1",
			protoMajor:   1,
			protoMinor:   1,
			expectedLine: "GET /test HTTP/1.1\r\n",
		},
		{
			name:         "HTTP/2.0",
			protoMajor:   2,
			protoMinor:   0,
			expectedLine: "GET /test HTTP/2.0\r\n",
		},
		{
			name:         "Default fallback when proto is zero",
			protoMajor:   0,
			protoMinor:   0,
			expectedLine: "GET /test HTTP/1.1\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test packet connection
			testPC := newTestPacketConnection()

			// Create HTTP handler
			handler := &httpHandler{}

			// Create test HTTP request with specific protocol version
			req := &http.Request{
				Method:     "GET",
				URL:        &url.URL{Path: "/test"},
				ProtoMajor: tt.protoMajor,
				ProtoMinor: tt.protoMinor,
				Header:     make(http.Header),
				Host:       "example.com",
			}

			// Call sendInitialHTTPRequest
			err := handler.sendInitialHTTPRequest(testPC, req, "localhost:8080")
			if err != nil {
				t.Fatalf("sendInitialHTTPRequest failed: %v", err)
			}

			// Verify that exactly one packet was sent
			if len(testPC.sentPackets) != 1 {
				t.Fatalf("Expected 1 packet to be sent, got %d", len(testPC.sentPackets))
			}

			packet := testPC.sentPackets[0]
			sentData := string(packet.Data)

			// Verify the request line contains the correct HTTP version
			if !strings.HasPrefix(sentData, tt.expectedLine) {
				t.Errorf("Expected request to start with %q, but got %q", tt.expectedLine, sentData[:len(tt.expectedLine)])
			}

			// Verify Host header is present
			if !strings.Contains(sentData, "Host: example.com\r\n") {
				t.Error("Expected Host header to be present in the request")
			}

			// Verify target address is set
			if packet.TargetAddress != "localhost:8080" {
				t.Errorf("Expected TargetAddress to be 'localhost:8080', got %q", packet.TargetAddress)
			}
		})
	}
}
