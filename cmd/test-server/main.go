package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/xuezhaojun/multiclustertunnel/pkg/server"
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
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Create hub server with both HTTP and gRPC
	config := &server.Config{
		GRPCListenAddress: *grpcAddr,
		HTTPListenAddress: *httpAddr,
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
	} else {
		klog.InfoS("HTTP TLS not configured - using insecure connection")
	}
	hubServer, err := server.New(config, &TestClusterNameParser{})
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

type TestClusterNameParser struct{}

func (p *TestClusterNameParser) ParseClusterName(r *http.Request) (string, error) {
	return "test-cluster", nil
}
