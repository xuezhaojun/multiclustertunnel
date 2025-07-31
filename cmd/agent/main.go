package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/xuezhaojun/multiclustertunnel/pkg/agent"
)

func main() {
	// Command line flags
	var (
		hubAddress    = flag.String("hub-address", "localhost:8443", "Address of the hub server")
		clusterName   = flag.String("cluster-name", "", "Name of the managed cluster (required)")
		udsSocketPath = flag.String("uds-socket-path", "/tmp/multiclustertunnel.sock", "Path to Unix Domain Socket")
		insecure      = flag.Bool("insecure", false, "Disable TLS certificate verification (for testing only)")
		hubKubeConfig = flag.String("hub-kubeconfig", "", "Path to hub cluster kubeconfig file (required)")
	)

	klog.InitFlags(nil)
	flag.Parse()

	if *clusterName == "" {
		klog.ErrorS(nil, "cluster-name is required")
		os.Exit(1)
	}

	if *hubKubeConfig == "" {
		klog.ErrorS(nil, "hub-kubeconfig is required")
		os.Exit(1)
	}

	klog.InfoS("Starting multiclustertunnel agent",
		"hub_address", *hubAddress,
		"cluster_name", *clusterName,
		"uds_socket_path", *udsSocketPath,
		"insecure", *insecure)

	// Create agent configuration
	config := &agent.Config{
		HubAddress:    *hubAddress,
		ClusterName:   *clusterName,
		UDSSocketPath: *udsSocketPath,
	}

	// Configure TLS
	if *insecure {
		// Use insecure connection (no TLS) for testing only
		config.DialOptions = append(config.DialOptions,
			grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
		klog.InfoS("Using insecure connection (no TLS) - for testing only")
	} else {
		// Use TLS with proper certificate verification (default)
		tlsConfig := &tls.Config{}
		config.DialOptions = append(config.DialOptions,
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		klog.InfoS("Using TLS with certificate verification enabled")
	}

	// Create Kubernetes clients for RequestProcessor
	var hubKubeClient, managedClusterKubeClient kubernetes.Interface

	// Create hub cluster client
	hubConfig, err := clientcmd.BuildConfigFromFlags("", *hubKubeConfig)
	if err != nil {
		klog.ErrorS(err, "Failed to build hub kubeconfig")
		os.Exit(1)
	}
	hubKubeClient, err = kubernetes.NewForConfig(hubConfig)
	if err != nil {
		klog.ErrorS(err, "Failed to create hub Kubernetes client")
		os.Exit(1)
	}
	klog.InfoS("Hub Kubernetes client created from kubeconfig", "kubeconfig", *hubKubeConfig)

	// Create managed cluster client (in-cluster config)
	managedClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.ErrorS(err, "Failed to get in-cluster config for managed cluster")
		os.Exit(1)
	}
	managedClusterKubeClient, err = kubernetes.NewForConfig(managedClusterConfig)
	if err != nil {
		klog.ErrorS(err, "Failed to create managed cluster Kubernetes client")
		os.Exit(1)
	}
	klog.InfoS("Managed cluster Kubernetes client created from in-cluster config")

	// Create default implementations of the interfaces
	requestProcessor := agent.NewRequestProcessorImplt(hubKubeClient, managedClusterKubeClient)

	certificateProvider := &agent.CertificateProviderImplt{}

	router := &agent.RouterImpl{}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create the agent with default implementations
	agentClient := agent.New(ctx, config, requestProcessor, certificateProvider, router)

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start agent in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- agentClient.Run(ctx)
	}()

	klog.InfoS("Agent started successfully")

	// Wait for shutdown signal or error
	select {
	case <-sigCh:
		klog.InfoS("Received shutdown signal, stopping agent...")
		cancel()
	case err := <-errCh:
		if err != nil {
			klog.ErrorS(err, "Agent stopped with error")
			os.Exit(1)
		}
	}

	klog.InfoS("Agent stopped")
}
