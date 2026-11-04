package oci

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// registryAPI answers /v2/ like a registry: with no credentials configured it
// is open, otherwise it demands basic auth for that user and password.
func registryAPI(user, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/" {
			http.NotFound(w, r)
			return
		}
		if user != "" {
			u, p, ok := r.BasicAuth()
			if !ok || u != user || p != password {
				w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}

func hostOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()

	return strings.TrimPrefix(strings.TrimPrefix(srv.URL, "https://"), "http://")
}

func TestVerifyLogin(t *testing.T) {
	protected := httptest.NewServer(registryAPI("admin", "right"))
	t.Cleanup(protected.Close)
	open := httptest.NewServer(registryAPI("", ""))
	t.Cleanup(open.Close)

	tests := []struct {
		name         string
		srv          *httptest.Server
		user, pass   string
		wantVerified bool
		wantKind     Kind
	}{
		{name: "right credentials", srv: protected, user: "admin", pass: "right", wantVerified: true},
		{name: "wrong password", srv: protected, user: "admin", pass: "wrong", wantKind: KindAuthRejected},
		{name: "wrong user", srv: protected, user: "someone", pass: "right", wantKind: KindAuthRejected},
		{name: "registry that needs no credentials cannot verify them", srv: open, user: "admin", pass: "whatever"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verified, err := VerifyLogin(context.Background(), hostOf(t, tt.srv),
				ClientOptions{Username: tt.user, Password: tt.pass, PlainHTTP: true})

			require.Equal(t, tt.wantVerified, verified)
			if tt.wantKind == 0 {
				require.NoError(t, err)
				return
			}
			var registryErr *RegistryError
			require.ErrorAs(t, err, &registryErr)
			require.Equal(t, tt.wantKind, registryErr.Kind)
		})
	}
}

func TestVerifyLoginNeverSendsTheWrongCredentialsAnonymously(t *testing.T) {
	var sawCredentials []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); ok {
			sawCredentials = append(sawCredentials, u+":"+p)
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	_, err := VerifyLogin(context.Background(), hostOf(t, srv), ClientOptions{Username: "u", Password: "p", PlainHTTP: true})

	require.Error(t, err)
	require.NotEmpty(t, sawCredentials, "the credentials are sent to the registry being logged in to")
	for _, c := range sawCredentials {
		require.Equal(t, "u:p", c)
	}
}

func TestVerifyLoginOverTLS(t *testing.T) {
	srv := httptest.NewTLSServer(registryAPI("admin", "right"))
	t.Cleanup(srv.Close)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	host := hostOf(t, srv)

	t.Run("with the CA", func(t *testing.T) {
		verified, err := VerifyLogin(context.Background(), host, ClientOptions{Username: "admin", Password: "right", CAPEM: caPEM})

		require.NoError(t, err)
		require.True(t, verified)
	})

	t.Run("without the CA", func(t *testing.T) {
		_, err := VerifyLogin(context.Background(), host, ClientOptions{Username: "admin", Password: "right"})

		var registryErr *RegistryError
		require.ErrorAs(t, err, &registryErr)
		require.Equal(t, KindUntrustedCertificate, registryErr.Kind)
	})
}

func TestVerifyLoginTransportMismatches(t *testing.T) {
	tlsSrv := httptest.NewTLSServer(registryAPI("admin", "right"))
	t.Cleanup(tlsSrv.Close)
	httpSrv := httptest.NewServer(registryAPI("admin", "right"))
	t.Cleanup(httpSrv.Close)

	t.Run("plain HTTP against a TLS server", func(t *testing.T) {
		_, err := VerifyLogin(context.Background(), hostOf(t, tlsSrv), ClientOptions{Username: "admin", Password: "right", PlainHTTP: true})

		require.Error(t, err)
	})

	t.Run("HTTPS against a plain HTTP server", func(t *testing.T) {
		_, err := VerifyLogin(context.Background(), hostOf(t, httpSrv), ClientOptions{Username: "admin", Password: "right"})

		var registryErr *RegistryError
		require.ErrorAs(t, err, &registryErr)
		require.Equal(t, KindHTTPOnly, registryErr.Kind)
	})
}

func TestVerifyLoginUnreachable(t *testing.T) {
	_, err := VerifyLogin(context.Background(), "127.0.0.1:1", ClientOptions{Username: "u", Password: "p", PlainHTTP: true})

	var registryErr *RegistryError
	require.ErrorAs(t, err, &registryErr)
	require.Equal(t, KindUnreachable, registryErr.Kind)
}

func TestVerifyLoginRejectsBadOptions(t *testing.T) {
	tests := []struct {
		name string
		opts ClientOptions
	}{
		{name: "no credentials", opts: ClientOptions{PlainHTTP: true}},
		{name: "username only", opts: ClientOptions{Username: "u", PlainHTTP: true}},
		{name: "CA certificate with plain HTTP", opts: ClientOptions{Username: "u", Password: "p", PlainHTTP: true, CAPEM: []byte("x")}},
		{name: "CA that is not PEM", opts: ClientOptions{Username: "u", Password: "p", CAPEM: []byte("not a certificate")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verified, err := VerifyLogin(context.Background(), "127.0.0.1:1", tt.opts)

			require.Error(t, err)
			require.False(t, verified)
		})
	}
}
