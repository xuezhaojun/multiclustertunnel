package e2e

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuezhaojun/multiclustertunnel/e2e/utils"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

var (
	testenv env.Environment

	// Command line flags for test configuration
	serverImage = flag.String("server-image", "", "Server docker image for testing")
	agentImage  = flag.String("agent-image", "", "Agent docker image for testing")
	kindImage   = flag.String("kind-image", "kindest/node:v1.30.2", "Kind node image")
	keepCluster = flag.Bool("keep-cluster", false, "Keep the Kind cluster after tests complete")
)

const (
	// Cluster and namespace configuration
	kindClusterName = "mctunnel-e2e"
	hubNamespace    = "mctunnel-hub"
	agentNamespace  = "mctunnel-agent"

	// Test timeouts
	clusterReadyTimeout = 5 * time.Minute
	deploymentTimeout   = 3 * time.Minute
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Validate required parameters
	if *serverImage == "" || *agentImage == "" {
		log.Fatalf("must provide both -server-image and -agent-image flags")
	}

	log.Printf("Starting e2e tests with configuration:")
	log.Printf("  Server Image: %s", *serverImage)
	log.Printf("  Agent Image: %s", *agentImage)
	log.Printf("  Kind Image: %s", *kindImage)
	log.Printf("  Keep Cluster: %v", *keepCluster)

	// Create test environment
	testenv = env.New()
	kindCluster := kind.NewCluster(kindClusterName).WithOpts(kind.WithImage(*kindImage))

	// Setup test environment
	testenv.Setup(
		// Create Kind cluster with configuration
		envfuncs.CreateClusterWithConfig(kindCluster, kindClusterName, "e2e/templates/kind.config"),

		// Load Docker images into cluster
		envfuncs.LoadImageToCluster(kindClusterName, *serverImage),
		envfuncs.LoadImageToCluster(kindClusterName, *agentImage),

		// Initialize utilities
		initializeTestUtilities,

		// Setup test resources
		setupTestNamespaces,
		setupCertificates,
		setupRBACResources,
		waitForClusterReady,
	)

	// Cleanup environment (unless keeping cluster for debugging)
	if !*keepCluster {
		testenv.Finish(
			cleanupTestResources,
			envfuncs.DestroyCluster(kindClusterName),
		)
	} else {
		testenv.Finish(
			func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
				log.Printf("Keeping cluster %s for debugging", kindClusterName)
				log.Printf("Access cluster with: kubectl --context kind-%s", kindClusterName)
				return ctx, nil
			},
		)
	}

	// Run tests
	os.Exit(testenv.Run(m))
}

// setupTestNamespaces creates the required namespaces for testing
func setupTestNamespaces(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	log.Printf("Setting up test namespaces...")

	// Create hub namespace
	if err := createNamespaceFromTemplate(ctx, cfg, "namespaces/hub-namespace.yaml", map[string]interface{}{
		"Name": hubNamespace,
	}); err != nil {
		return ctx, err
	}

	// Create agent namespace
	if err := createNamespaceFromTemplate(ctx, cfg, "namespaces/agent-namespace.yaml", map[string]interface{}{
		"Name": agentNamespace,
	}); err != nil {
		return ctx, err
	}

	log.Printf("Test namespaces created successfully")
	return ctx, nil
}

// setupCertificates generates and distributes certificates for secure communication
func setupCertificates(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	log.Printf("Setting up certificates...")

	// Generate certificates
	certs, err := generateTestCertificates()
	if err != nil {
		return ctx, err
	}

	// Create certificate secrets in hub namespace
	if err := createCertificateSecret(ctx, cfg, hubNamespace, "mctunnel-ca-secret", certs.CACert, ""); err != nil {
		return ctx, err
	}
	if err := createCertificateSecret(ctx, cfg, hubNamespace, "mctunnel-server-secret", certs.ServerCert, certs.ServerKey); err != nil {
		return ctx, err
	}

	// Create certificate secrets in agent namespace
	if err := createCertificateSecret(ctx, cfg, agentNamespace, "mctunnel-ca-secret", certs.CACert, ""); err != nil {
		return ctx, err
	}
	if err := createCertificateSecret(ctx, cfg, agentNamespace, "mctunnel-client-secret", certs.ClientCert, certs.ClientKey); err != nil {
		return ctx, err
	}

	// Create hub kubeconfig secret in agent namespace
	if err := createHubKubeConfigSecret(ctx, cfg, agentNamespace, "hub-kubeconfig"); err != nil {
		return ctx, err
	}

	log.Printf("Certificates setup completed")
	return ctx, nil
}

// setupRBACResources creates the required RBAC resources
func setupRBACResources(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	log.Printf("Setting up RBAC resources...")

	// Create server RBAC resources
	serverRBACTemplates := []string{
		"server/serviceaccount.yaml",
		"server/clusterrole.yaml",
		"server/clusterrolebinding.yaml",
	}

	for _, template := range serverRBACTemplates {
		if err := applyTemplate(ctx, cfg, template, map[string]interface{}{
			"Namespace": hubNamespace,
			"Name":      "mctunnel-server",
		}); err != nil {
			return ctx, err
		}
	}

	// Create agent RBAC resources
	agentRBACTemplates := []string{
		"agent/serviceaccount.yaml",
		"agent/clusterrole.yaml",
		"agent/clusterrolebinding.yaml",
	}

	for _, template := range agentRBACTemplates {
		if err := applyTemplate(ctx, cfg, template, map[string]interface{}{
			"Namespace": agentNamespace,
			"Name":      "mctunnel-agent",
		}); err != nil {
			return ctx, err
		}
	}

	log.Printf("RBAC resources setup completed")
	return ctx, nil
}

// waitForClusterReady waits for the cluster to be fully ready
func waitForClusterReady(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	log.Printf("Waiting for cluster to be ready...")

	// Add any specific readiness checks here
	// For now, just wait a bit for everything to settle
	time.Sleep(10 * time.Second)

	log.Printf("Cluster is ready")
	return ctx, nil
}

// cleanupTestResources cleans up test resources before cluster destruction
func cleanupTestResources(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	log.Printf("Cleaning up test resources...")

	// Cleanup will be handled by cluster destruction
	// Add any specific cleanup logic here if needed

	log.Printf("Test resources cleanup completed")
	return ctx, nil
}

// initializeTestUtilities initializes global utilities for testing
func initializeTestUtilities(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	log.Printf("Initializing test utilities...")

	// Get current working directory and set up template directory
	templateDir := filepath.Join("e2e", "templates")
	utils.GlobalRenderer = utils.NewTemplateRenderer(templateDir)
	utils.InitializeClusterManager(cfg)

	log.Printf("Test utilities initialized")
	return ctx, nil
}

// createNamespaceFromTemplate creates a namespace from a template
func createNamespaceFromTemplate(ctx context.Context, cfg *envconf.Config, templateFile string, params map[string]interface{}) error {
	return utils.ApplyTemplate(ctx, cfg, templateFile, params)
}

// generateTestCertificates generates certificates for testing
func generateTestCertificates() (*utils.CertificateBundle, error) {
	return utils.GenerateTestCertificates()
}

// createCertificateSecret creates a certificate secret
func createCertificateSecret(ctx context.Context, cfg *envconf.Config, namespace, name, cert, key string) error {
	return utils.CreateCertificateSecret(ctx, cfg, namespace, name, cert, key)
}

// createHubKubeConfigSecret creates a hub kubeconfig secret
func createHubKubeConfigSecret(ctx context.Context, cfg *envconf.Config, namespace, name string) error {
	return utils.CreateHubKubeConfigSecret(ctx, cfg, namespace, name)
}

// applyTemplate applies a template with parameters
func applyTemplate(ctx context.Context, cfg *envconf.Config, templateFile string, params map[string]interface{}) error {
	return utils.ApplyTemplate(ctx, cfg, templateFile, params)
}
