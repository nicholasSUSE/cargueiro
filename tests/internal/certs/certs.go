// Package certs generates the throwaway TLS material used by the e2e registry:
// a self-signed CA and a server certificate for localhost.
package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// File names written by Generate.
const (
	CAFile     = "ca.crt"
	ServerCert = "server.crt"
	ServerKey  = "server.key"
)

const validity = 30 * 24 * time.Hour

// Generate writes a CA certificate and a server certificate and key into dir.
// The server certificate is valid for localhost and 127.0.0.1 and signed by the
// CA. dir is created if it does not exist.
func Generate(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate CA key: %w", err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "cargueiro e2e CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(validity),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create CA certificate: %w", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return fmt.Errorf("parse CA certificate: %w", err)
	}

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate server key: %w", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create server certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return fmt.Errorf("marshal server key: %w", err)
	}

	// The registry container runs as a different user, so the key must be
	// world-readable. It protects a throwaway registry on localhost only.
	files := []struct {
		name string
		typ  string
		der  []byte
	}{
		{CAFile, "CERTIFICATE", caDER},
		{ServerCert, "CERTIFICATE", serverDER},
		{ServerKey, "EC PRIVATE KEY", keyDER},
	}
	for _, f := range files {
		data := pem.EncodeToMemory(&pem.Block{Type: f.typ, Bytes: f.der})
		if err := os.WriteFile(filepath.Join(dir, f.name), data, 0o644); err != nil { //nolint:gosec // see comment above
			return fmt.Errorf("write %s: %w", f.name, err)
		}
	}
	return nil
}
