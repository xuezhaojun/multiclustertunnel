package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

var (
	addr = flag.String("addr", ":9090", "HTTP server address")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.InfoS("Starting test-simple-server", "address", *addr)

	// Create HTTP server with simple endpoints
	mux := http.NewServeMux()

	// Hello World endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		klog.InfoS("Received request", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)

		response := fmt.Sprintf("Hello World from test-simple-server!\nTime: %s\nPath: %s\nMethod: %s\n",
			time.Now().Format(time.RFC3339), r.URL.Path, r.Method)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		klog.V(4).InfoS("Health check request", "remote_addr", r.RemoteAddr)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// API endpoint for testing
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		klog.InfoS("API request", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"message": "%s", "path": "%s", "method": "%s", "timestamp": "%s"}`,
			"API Response from test-simple-server!", r.URL.Path, r.Method, time.Now().Format(time.RFC3339))))
	})

	server := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		klog.InfoS("HTTP server started", "address", *addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "HTTP server failed")
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-sigCh
	klog.InfoS("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		klog.ErrorS(err, "Server shutdown failed")
		os.Exit(1)
	}

	klog.InfoS("Server stopped")
}
