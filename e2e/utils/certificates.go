package utils

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// CertificateBundle contains all certificates needed for testing
type CertificateBundle struct {
	CACert     string
	CAKey      string
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
}

// GenerateTestCertificates generates a complete set of certificates for e2e testing
func GenerateTestCertificates() (*CertificateBundle, error) {
	// Generate CA certificate and key
	caCert, caKey, err := generateCACertificate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA certificate: %w", err)
	}

	// Generate server certificate and key
	serverCert, serverKey, err := generateServerCertificate(caCert, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server certificate: %w", err)
	}

	// Generate client certificate and key
	clientCert, clientKey, err := generateClientCertificate(caCert, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client certificate: %w", err)
	}

	return &CertificateBundle{
		CACert:     encodeCertToPEM(caCert),
		CAKey:      encodeKeyToPEM(caKey),
		ServerCert: encodeCertToPEM(serverCert),
		ServerKey:  encodeKeyToPEM(serverKey),
		ClientCert: encodeCertToPEM(clientCert),
		ClientKey:  encodeKeyToPEM(clientKey),
	}, nil
}

// generateCACertificate generates a CA certificate and private key
func generateCACertificate() (*x509.Certificate, *rsa.PrivateKey, error) {
	// Generate private key
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"MultiClusterTunnel E2E Test"},
			Country:       []string{"US"},
			Province:      []string{"CA"},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
			CommonName:    "MultiClusterTunnel E2E CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour), // Valid for 24 hours
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	return cert, caKey, nil
}

// generateServerCertificate generates a server certificate signed by the CA
func generateServerCertificate(caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Generate private key
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template with SAN entries for Kubernetes services
	dnsNames := []string{
		"mctunnel-server",
		"mctunnel-server.mctunnel-hub",
		"mctunnel-server.mctunnel-hub.svc",
		"mctunnel-server.mctunnel-hub.svc.cluster.local",
		"localhost",
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"MultiClusterTunnel E2E Test"},
			Country:      []string{"US"},
			Province:     []string{"CA"},
			Locality:     []string{"San Francisco"},
			CommonName:   "mctunnel-server",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),
		DNSNames:  dnsNames,
		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv6loopback,
		},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	return cert, serverKey, nil
}

// generateClientCertificate generates a client certificate signed by the CA
func generateClientCertificate(caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Generate private key
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization: []string{"MultiClusterTunnel E2E Test"},
			Country:      []string{"US"},
			Province:     []string{"CA"},
			Locality:     []string{"San Francisco"},
			CommonName:   "mctunnel-client",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	return cert, clientKey, nil
}

// encodeCertToPEM encodes a certificate to PEM format
func encodeCertToPEM(cert *x509.Certificate) string {
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
	return string(certPEM)
}

// encodeKeyToPEM encodes a private key to PEM format
func encodeKeyToPEM(key *rsa.PrivateKey) string {
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return string(keyPEM)
}

// CreateCertificateSecret creates a Kubernetes secret with certificate data
func CreateCertificateSecret(ctx context.Context, cfg *envconf.Config, namespace, name, cert, key string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "multiclustertunnel",
				"app.kubernetes.io/part-of":   "multiclustertunnel-e2e",
				"app.kubernetes.io/component": "certificates",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt": []byte(cert),
		},
	}

	// Add private key if provided
	if key != "" {
		secret.Data["tls.crt"] = []byte(cert)
		secret.Data["tls.key"] = []byte(key)
	}

	return cfg.Client().Resources().Create(ctx, secret)
}

// CreateCASecret creates a CA certificate secret using template
func CreateCASecret(ctx context.Context, cfg *envconf.Config, namespace, name string, certs *CertificateBundle) error {
	params := map[string]interface{}{
		"Name":      name,
		"Namespace": namespace,
		"CACert":    certs.CACert,
		"CAKey":     certs.CAKey,
	}
	return ApplyTemplate(ctx, cfg, "certificates/ca-secret.yaml", params)
}

// CreateServerSecret creates a server certificate secret using template
func CreateServerSecret(ctx context.Context, cfg *envconf.Config, namespace, name string, certs *CertificateBundle) error {
	params := map[string]interface{}{
		"Name":       name,
		"Namespace":  namespace,
		"ServerCert": certs.ServerCert,
		"ServerKey":  certs.ServerKey,
		"CACert":     certs.CACert,
	}
	return ApplyTemplate(ctx, cfg, "certificates/server-secret.yaml", params)
}

// CreateClientSecret creates a client certificate secret using template
func CreateClientSecret(ctx context.Context, cfg *envconf.Config, namespace, name string, certs *CertificateBundle) error {
	params := map[string]interface{}{
		"Name":       name,
		"Namespace":  namespace,
		"ClientCert": certs.ClientCert,
		"ClientKey":  certs.ClientKey,
		"CACert":     certs.CACert,
	}
	return ApplyTemplate(ctx, cfg, "certificates/client-secret.yaml", params)
}

// getKubeConfigContent gets the kubeconfig content for the current cluster
func getKubeConfigContent(cfg *envconf.Config) (string, error) {
	// In e2e tests, we can get the kubeconfig from the REST config
	restConfig := cfg.Client().RESTConfig()

	// Create a kubeconfig structure
	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
    insecure-skip-tls-verify: true
  name: kind-mctunnel-e2e
contexts:
- context:
    cluster: kind-mctunnel-e2e
    user: kind-mctunnel-e2e
  name: kind-mctunnel-e2e
current-context: kind-mctunnel-e2e
users:
- name: kind-mctunnel-e2e
  user:
    token: %s
`, restConfig.Host, restConfig.BearerToken)

	return kubeconfig, nil
}

// CreateHubKubeConfigSecret creates a hub kubeconfig secret using template
func CreateHubKubeConfigSecret(ctx context.Context, cfg *envconf.Config, namespace, name string) error {
	// Get the current kubeconfig from the environment
	kubeconfig, err := getKubeConfigContent(cfg)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig content: %w", err)
	}

	params := map[string]interface{}{
		"Name":       name,
		"Namespace":  namespace,
		"KubeConfig": kubeconfig,
	}
	return ApplyTemplate(ctx, cfg, "certificates/hub-kubeconfig-secret.yaml", params)
}
