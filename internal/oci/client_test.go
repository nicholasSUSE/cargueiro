package oci

import (
	"crypto/tls"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClientOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    ClientOptions
		wantErr string
	}{
		{name: "anonymous", opts: ClientOptions{}},
		{name: "credentials", opts: ClientOptions{Username: "u", Password: "p"}},
		{name: "plain HTTP", opts: ClientOptions{PlainHTTP: true}},
		{name: "username without password", opts: ClientOptions{Username: "u"}, wantErr: "together"},
		{name: "password without username", opts: ClientOptions{Password: "p"}, wantErr: "together"},
		{name: "plain HTTP with CA file", opts: ClientOptions{PlainHTTP: true, CAFile: "ca.pem"}, wantErr: "plain HTTP"},
		{name: "missing CA file", opts: ClientOptions{CAFile: filepath.Join(t.TempDir(), "absent.pem")}, wantErr: "read CA file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClient(tt.opts)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
		})
	}
}

func TestNewClientRejectsConflictsAsInvalidOptions(t *testing.T) {
	_, err := NewClient(ClientOptions{Username: "u"})
	require.ErrorIs(t, err, ErrInvalidOptions)

	_, err = NewClient(ClientOptions{PlainHTTP: true, CAFile: "x"})
	require.ErrorIs(t, err, ErrInvalidOptions)
}

func TestTLSConfigWithCA(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	// The test server's certificate is self-signed, so it acts as its own CA.
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	require.NoError(t, os.WriteFile(caFile, caPEM, 0o600))

	get := func(t *testing.T, cfg *tls.Config) error {
		t.Helper()
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
		resp, err := client.Get(srv.URL)
		if err == nil {
			require.NoError(t, resp.Body.Close())
		}
		return err
	}

	t.Run("trusted with the CA file", func(t *testing.T) {
		cfg, err := tlsConfigWithCA(caFile)
		require.NoError(t, err)

		require.NoError(t, get(t, cfg))
	})

	t.Run("untrusted without it", func(t *testing.T) {
		cfg, err := tlsConfigWithCA("")
		require.NoError(t, err)

		require.ErrorContains(t, get(t, cfg), "certificate")
	})

	t.Run("minimum TLS version is 1.2", func(t *testing.T) {
		cfg, err := tlsConfigWithCA(caFile)
		require.NoError(t, err)

		require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	})

	t.Run("file without a certificate", func(t *testing.T) {
		junk := filepath.Join(t.TempDir(), "junk.pem")
		require.NoError(t, os.WriteFile(junk, []byte("nope"), 0o600))

		_, err := tlsConfigWithCA(junk)

		require.ErrorIs(t, err, ErrInvalidOptions)
	})
}
