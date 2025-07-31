package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/xuezhaojun/multiclustertunnel/e2e/utils"
)

func main() {
	var (
		outputDir = flag.String("output-dir", "e2e/certs", "Directory to output certificates")
		help      = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		fmt.Println("Certificate Generator for MultiClusterTunnel E2E Testing")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Printf("  %s [flags]\n", os.Args[0])
		fmt.Println("")
		fmt.Println("Flags:")
		flag.PrintDefaults()
		fmt.Println("")
		fmt.Println("This tool generates CA, server, and client certificates for secure")
		fmt.Println("gRPC communication in e2e tests.")
		return
	}

	log.Printf("Generating certificates for MultiClusterTunnel e2e testing...")
	log.Printf("Output directory: %s", *outputDir)

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Generate certificates
	certs, err := utils.GenerateTestCertificates()
	if err != nil {
		log.Fatalf("Failed to generate certificates: %v", err)
	}

	// Write CA certificate and key
	caCertPath := filepath.Join(*outputDir, "ca-cert.pem")
	if err := writeFile(caCertPath, certs.CACert, 0644); err != nil {
		log.Fatalf("Failed to write CA certificate: %v", err)
	}
	log.Printf("✓ CA certificate written to: %s", caCertPath)

	caKeyPath := filepath.Join(*outputDir, "ca-key.pem")
	if err := writeFile(caKeyPath, certs.CAKey, 0600); err != nil {
		log.Fatalf("Failed to write CA key: %v", err)
	}
	log.Printf("✓ CA private key written to: %s", caKeyPath)

	// Write server certificate and key
	serverCertPath := filepath.Join(*outputDir, "server-cert.pem")
	if err := writeFile(serverCertPath, certs.ServerCert, 0644); err != nil {
		log.Fatalf("Failed to write server certificate: %v", err)
	}
	log.Printf("✓ Server certificate written to: %s", serverCertPath)

	serverKeyPath := filepath.Join(*outputDir, "server-key.pem")
	if err := writeFile(serverKeyPath, certs.ServerKey, 0600); err != nil {
		log.Fatalf("Failed to write server key: %v", err)
	}
	log.Printf("✓ Server private key written to: %s", serverKeyPath)

	// Write client certificate and key
	clientCertPath := filepath.Join(*outputDir, "client-cert.pem")
	if err := writeFile(clientCertPath, certs.ClientCert, 0644); err != nil {
		log.Fatalf("Failed to write client certificate: %v", err)
	}
	log.Printf("✓ Client certificate written to: %s", clientCertPath)

	clientKeyPath := filepath.Join(*outputDir, "client-key.pem")
	if err := writeFile(clientKeyPath, certs.ClientKey, 0600); err != nil {
		log.Fatalf("Failed to write client key: %v", err)
	}
	log.Printf("✓ Client private key written to: %s", clientKeyPath)

	log.Printf("Certificate generation completed successfully!")
	log.Printf("")
	log.Printf("Generated files:")
	log.Printf("  • CA Certificate: %s", caCertPath)
	log.Printf("  • CA Private Key: %s", caKeyPath)
	log.Printf("  • Server Certificate: %s", serverCertPath)
	log.Printf("  • Server Private Key: %s", serverKeyPath)
	log.Printf("  • Client Certificate: %s", clientCertPath)
	log.Printf("  • Client Private Key: %s", clientKeyPath)
	log.Printf("")
	log.Printf("These certificates are valid for 24 hours and are intended for testing only.")
	log.Printf("DO NOT use these certificates in production!")
}

// writeFile writes content to a file with specified permissions
func writeFile(path, content string, perm os.FileMode) error {
	return os.WriteFile(path, []byte(content), perm)
}
