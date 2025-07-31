package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"

	"github.com/xuezhaojun/multiclustertunnel/pkg/server"
)

func main() {
	// Command line flags
	var (
		grpcAddr     = flag.String("grpc-address", ":8443", "gRPC server address for agent connections")
		httpAddr     = flag.String("http-address", ":8080", "HTTP server address for client requests")
		grpcCertFile = flag.String("grpc-cert-file", "", "Path to gRPC TLS certificate file")
		grpcKeyFile  = flag.String("grpc-key-file", "", "Path to gRPC TLS private key file")
		httpCertFile = flag.String("http-cert-file", "", "Path to HTTP TLS certificate file")
		httpKeyFile  = flag.String("http-key-file", "", "Path to HTTP TLS private key file")
	)

	klog.InitFlags(nil)
	flag.Parse()

	klog.InfoS("Starting multiclustertunnel server",
		"grpc_address", *grpcAddr,
		"http_address", *httpAddr,
		"grpc_tls_enabled", *grpcCertFile != "" && *grpcKeyFile != "",
		"http_tls_enabled", *httpCertFile != "" && *httpKeyFile != "")

	// Create server configuration
	config := &server.Config{
		GRPCListenAddress: *grpcAddr,
		HTTPListenAddress: *httpAddr,
	}

	// Configure gRPC TLS
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
		klog.InfoS("gRPC TLS not configured - using insecure connection")
	}

	// Configure HTTP TLS
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

	// Create default implementation of ClusterNameParser
	clusterNameParser := server.NewClusterNameParserImplt()

	// Create the server with default implementation
	hubServer, err := server.New(config, clusterNameParser)
	if err != nil {
		klog.ErrorS(err, "Failed to create hub server")
		os.Exit(1)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	klog.InfoS("Server started", "grpc_address", *grpcAddr, "http_address", *httpAddr)

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- hubServer.Run(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigCh:
		klog.InfoS("Received shutdown signal, stopping server...")
		cancel()
		hubServer.Shutdown(context.Background())
	case err := <-errCh:
		if err != nil {
			klog.ErrorS(err, "Server stopped with error")
			os.Exit(1)
		}
	}

	klog.InfoS("Server stopped")
}
