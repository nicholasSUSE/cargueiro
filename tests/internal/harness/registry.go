//go:build e2e

package harness

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Credentials of the e2e registries (see tests/data/registry/htpasswd).
const (
	RegistryUser     = "admin"
	RegistryPassword = "cargueiro-test"
)

const (
	defaultHTTPRegistry = "127.0.0.1:5050"
	defaultTLSRegistry  = "127.0.0.1:5051"
	dialTimeout         = 2 * time.Second
)

// HTTPRegistry returns the address of the plain-HTTP e2e registry. It fails the
// test when the registry is not running.
func HTTPRegistry(t *testing.T) string {
	t.Helper()

	return reachable(t, envOr("CARGUEIRO_TEST_REGISTRY_HTTP", defaultHTTPRegistry))
}

// TLSRegistry returns the address of the TLS e2e registry. It fails the test
// when the registry is not running.
func TLSRegistry(t *testing.T) string {
	t.Helper()

	return reachable(t, envOr("CARGUEIRO_TEST_REGISTRY_TLS", defaultTLSRegistry))
}

// RegistryCA returns the path of the CA certificate that signed the TLS
// registry's certificate.
func RegistryCA(t *testing.T) string {
	t.Helper()

	ca := filepath.Join(RepoRoot(t), ".tmp", "registry", "certs", "ca.crt")
	require.FileExists(t, ca, "registry CA not found: run `make registry-up`")
	return ca
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func reachable(t *testing.T, addr string) string {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	require.NoErrorf(t, err, "registry %s is not reachable: run `make registry-up` (`make test-e2e` starts it for you)", addr)
	require.NoError(t, conn.Close())
	return addr
}
