package certs

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "certs")

	require.NoError(t, Generate(dir))

	caPEM, err := os.ReadFile(filepath.Join(dir, CAFile))
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(caPEM))

	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, ServerCert), filepath.Join(dir, ServerKey))
	require.NoError(t, err)
	server, err := x509.ParseCertificate(pair.Certificate[0])
	require.NoError(t, err)

	tests := []struct {
		name    string
		host    string
		roots   *x509.CertPool
		wantErr bool
	}{
		{name: "localhost", host: "localhost", roots: pool},
		{name: "loopback IP", host: "127.0.0.1", roots: pool},
		{name: "other host is rejected", host: "example.com", roots: pool, wantErr: true},
		{name: "unknown CA is rejected", host: "localhost", roots: x509.NewCertPool(), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.Verify(x509.VerifyOptions{
				DNSName:   tt.host,
				Roots:     tt.roots,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGenerateIsRepeatable(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, Generate(dir))
	require.NoError(t, Generate(dir), "regenerating over existing files must work")
}
