package oci

import (
	"crypto/tls"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
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
		{name: "plain HTTP with a CA certificate", opts: ClientOptions{PlainHTTP: true, CAPEM: []byte("x")}, wantErr: "plain HTTP"},
		{name: "CA that is not PEM", opts: ClientOptions{CAPEM: []byte("not a certificate")}, wantErr: "no PEM certificate"},
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

	_, err = NewClient(ClientOptions{PlainHTTP: true, CAPEM: []byte("x")})
	require.ErrorIs(t, err, ErrInvalidOptions)
}

func TestTLSConfigWithCA(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	// The test server's certificate is self-signed, so it acts as its own CA.
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})

	get := func(t *testing.T, cfg *tls.Config) error {
		t.Helper()
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
		resp, err := client.Get(srv.URL)
		if err == nil {
			require.NoError(t, resp.Body.Close())
		}
		return err
	}

	t.Run("trusted with the CA certificate", func(t *testing.T) {
		cfg, err := tlsConfigWithCA(caPEM)
		require.NoError(t, err)

		require.NoError(t, get(t, cfg))
	})

	t.Run("untrusted without it", func(t *testing.T) {
		cfg, err := tlsConfigWithCA(nil)
		require.NoError(t, err)

		require.ErrorContains(t, get(t, cfg), "certificate")
	})

	t.Run("minimum TLS version is 1.2", func(t *testing.T) {
		cfg, err := tlsConfigWithCA(caPEM)
		require.NoError(t, err)

		require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	})

	t.Run("text without a certificate", func(t *testing.T) {
		_, err := tlsConfigWithCA([]byte("nope"))

		require.ErrorIs(t, err, ErrInvalidOptions)
	})
}

func TestClientExists(t *testing.T) {
	const digest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
		case "/v2/team/fleet/manifests/1.0.0":
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", digest)
			w.Header().Set("Content-Length", "0")
		case "/v2/team/broken/manifests/1.0.0":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	host := srv.Listener.Addr().String()

	client, err := NewClient(ClientOptions{PlainHTTP: true})
	require.NoError(t, err)

	tests := []struct {
		name    string
		ref     string
		want    bool
		wantErr bool
	}{
		{
			name: "a stored chart",
			ref:  host + "/team/fleet:1.0.0",
			want: true,
		},
		{
			name: "another version",
			ref:  host + "/team/fleet:1.0.1",
			want: false,
		},
		{
			name: "a chart that is not there",
			ref:  host + "/team/other:1.0.0",
			want: false,
		},
		{
			name:    "a registry error is not an absence",
			ref:     host + "/team/broken:1.0.0",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.Exists(tt.ref)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
