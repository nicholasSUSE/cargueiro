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
	defaultOpenRegistry = "127.0.0.1:5052"
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

// OpenRegistry returns the address of the plain-HTTP e2e registry that asks for
// no password. It fails the test when the registry is not running.
func OpenRegistry(t *testing.T) string {
	t.Helper()

	return reachable(t, envOr("CARGUEIRO_TEST_REGISTRY_OPEN", defaultOpenRegistry))
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

// HelmPull fetches a chart from a registry with the helm CLI, as the e2e
// registry user, and returns the path of the pulled archive. When the pull
// fails it returns "" and the result.
func HelmPull(t *testing.T, address, path, chart, version string, flags []string) (string, Result) {
	t.Helper()

	dest, cfg := t.TempDir(), t.TempDir()
	args := []string{
		"pull", "oci://" + address + "/" + path + "/" + chart,
		"--version", version, "--destination", dest,
		"--username", RegistryUser, "--password", RegistryPassword,
	}
	args = append(args, flags...)

	res := HelmWith(t, Options{
		Dir: t.TempDir(),
		Env: []string{
			"HELM_REGISTRY_CONFIG=" + filepath.Join(cfg, "registry.json"),
			"HELM_REPOSITORY_CONFIG=" + filepath.Join(cfg, "repositories.yaml"),
			"HELM_REPOSITORY_CACHE=" + filepath.Join(cfg, "cache"),
		},
	}, args...)
	if res.ExitCode != 0 {
		return "", res
	}
	return filepath.Join(dest, chart+"-"+version+".tgz"), res
}
