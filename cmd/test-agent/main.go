package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/xuezhaojun/multiclustertunnel/pkg/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

func main() {
	var (
		hubAddress     = flag.String("hub-address", "localhost:8443", "Address of the hub server")
		clusterName    = flag.String("cluster-name", "test-cluster", "Name of the cluster")
		useInsecure    = flag.Bool("insecure", false, "Use insecure connection (no TLS)")
		skipTLSVerify  = flag.Bool("skip-tls-verify", false, "Skip TLS certificate verification (for testing)")
	)
	flag.Parse()

	klog.InitFlags(nil)
	klog.InfoS("Starting test-agent-client",
		"hubAddress", *hubAddress,
		"clusterName", *clusterName,
		"insecure", *useInsecure)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create agent configuration
	config := &agent.Config{
		HubAddress:  *hubAddress,
		ClusterName: *clusterName,
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

	// Create the agent client
	client := agent.NewClient(ctx, config)

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
