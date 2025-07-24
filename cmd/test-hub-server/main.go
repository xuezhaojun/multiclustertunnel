package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/xuezhaojun/multiclustertunnel/pkg/hub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

var (
	grpcAddr = flag.String("grpc-addr", ":8443", "gRPC server address")
	httpAddr = flag.String("http-addr", ":8080", "HTTP server address")

	// gRPC TLS configuration
	grpcCertFile = flag.String("grpc-cert-file", "", "Path to TLS certificate file for gRPC server")
	grpcKeyFile  = flag.String("grpc-key-file", "", "Path to TLS private key file for gRPC server")

	// HTTP TLS configuration
	httpCertFile = flag.String("http-cert-file", "", "Path to TLS certificate file for HTTP server")
	httpKeyFile  = flag.String("http-key-file", "", "Path to TLS private key file for HTTP server")

	// Legacy flags for backward compatibility (applies to both servers)
	certFile = flag.String("cert-file", "", "Path to TLS certificate file (applies to both gRPC and HTTP servers)")
	keyFile  = flag.String("key-file", "", "Path to TLS private key file (applies to both gRPC and HTTP servers)")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Create simple Router
	router := &SimpleRouter{}

	// Create hub server with both HTTP and gRPC
	config := &hub.Config{
		GRPCListenAddress: *grpcAddr,
		HTTPListenAddress: *httpAddr,
		Router:            router,
	}

	// Configure TLS for gRPC server
	if *grpcCertFile != "" && *grpcKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(*grpcCertFile, *grpcKeyFile)
		if err != nil {
			klog.ErrorS(err, "Failed to load gRPC TLS certificate")
			os.Exit(1)
		}

		grpcTLSConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.NoClientCert,
		}

		config.GRPCTLSConfig = grpcTLSConfig
		klog.InfoS("gRPC TLS enabled", "cert_file", *grpcCertFile, "key_file", *grpcKeyFile)
	} else if *certFile != "" && *keyFile != "" {
		// Legacy support: use common cert for gRPC
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			klog.ErrorS(err, "Failed to load TLS certificate")
			os.Exit(1)
		}

		grpcTLSConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.NoClientCert,
		}

		config.GRPCTLSConfig = grpcTLSConfig
		klog.InfoS("gRPC TLS enabled (legacy)", "cert_file", *certFile, "key_file", *keyFile)
	} else {
		// For development/testing - use insecure credentials for gRPC
		config.ServerOptions = []grpc.ServerOption{
			grpc.Creds(insecure.NewCredentials()),
		}
		klog.InfoS("gRPC TLS not configured - using insecure connection")
	}

	// Configure TLS for HTTP server
	if *httpCertFile != "" && *httpKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(*httpCertFile, *httpKeyFile)
		if err != nil {
			klog.ErrorS(err, "Failed to load HTTP TLS certificate")
			os.Exit(1)
		}

		httpTLSConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.NoClientCert,
		}

		config.HTTPTLSConfig = httpTLSConfig
		klog.InfoS("HTTP TLS enabled", "cert_file", *httpCertFile, "key_file", *httpKeyFile)
	} else if *certFile != "" && *keyFile != "" {
		// Legacy support: use common cert for HTTP
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			klog.ErrorS(err, "Failed to load TLS certificate")
			os.Exit(1)
		}

		httpTLSConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.NoClientCert,
		}

		config.HTTPTLSConfig = httpTLSConfig
		klog.InfoS("HTTP TLS enabled (legacy)", "cert_file", *certFile, "key_file", *keyFile)
	} else {
		klog.InfoS("HTTP TLS not configured - using insecure connection")
	}
	hubServer, err := hub.New(config)
	if err != nil {
		klog.ErrorS(err, "Failed to create hub server")
		os.Exit(1)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	klog.InfoS("Servers started", "grpc", *grpcAddr, "http", *httpAddr)

	// Start server (handles both HTTP and gRPC)
	go hubServer.Run(ctx)

	// Wait for shutdown signal
	<-sigCh
	klog.InfoS("Shutting down...")
	cancel()
	hubServer.Shutdown(context.Background())
}

// SimpleRouter implements hub.Router
type SimpleRouter struct{}

func (r *SimpleRouter) Parse(req *http.Request) (cluster, targetAddress string, err error) {
	// Extract cluster name from path: /cluster-name/service-path
	path := req.URL.Path[1:] // Remove leading /
	if len(path) == 0 {
		return "", "", fmt.Errorf("missing cluster name")
	}

	// Find first slash to separate cluster name from service path
	slashIdx := -1
	for i, c := range path {
		if c == '/' {
			slashIdx = i
			break
		}
	}

	if slashIdx > 0 {
		cluster = path[:slashIdx]
		// For this simple example, we'll route to a default service
		// The agent will receive the full HTTP request and forward it to the target service
		targetAddress = "localhost:8080"
	} else {
		// No service path, just cluster name
		cluster = path
		targetAddress = "localhost:8080"
	}

	if cluster == "" {
		return "", "", fmt.Errorf("missing cluster name")
	}

	return cluster, targetAddress, nil
}
