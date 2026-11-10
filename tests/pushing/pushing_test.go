//go:build e2e

// Package pushing holds the e2e tests of `cargueiro push`, run against the real
// binary and real local registries (see `make registry-up`).
//
// push has no transport flags: a registry served over plain HTTP or behind a
// private CA is set up by `registry login`, so every test that reaches one logs
// in first.
package pushing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

const fleetVersion = "110.0.3+up0.16.3"

// registryTag is how the registry stores fleetVersion: Helm turns "+" into "_".
const registryTag = "110.0.3_up0.16.3"

func fleetCharts() []string { return []string{"fleet-crd", "fleet", "fleet-agent"} }

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// repoPath returns a registry path unique to this run, so tests never see
// each other's charts.
func repoPath() string {
	return fmt.Sprintf("e2e/%d", time.Now().UnixNano())
}

func published(t *testing.T, chart string) string {
	t.Helper()

	return harness.FleetPublished(t, chart, fleetVersion)
}

func requireNoPassword(t *testing.T, res harness.Result) {
	t.Helper()

	require.NotContains(t, res.Stdout, harness.RegistryPassword)
	require.NotContains(t, res.Stderr, harness.RegistryPassword)
}

// pull fetches a chart with the helm CLI, which is the independent check that a
// push really stored something usable.
func pull(t *testing.T, tr harness.Transport, address, path, chart string) (string, harness.Result) {
	t.Helper()

	return harness.HelmPull(t, address, path, chart, fleetVersion, tr.HelmFlags(t))
}

func TestPushFleetCharts(t *testing.T) {
	for _, tr := range harness.Transports() {
		t.Run(tr.Name, func(t *testing.T) {
			s := harness.NewSession(t)
			address, path := s.LoginVia(t, tr), repoPath()

			for _, chart := range fleetCharts() {
				res := s.Push(t, address+"/"+path, published(t, chart))

				requireNoPassword(t, res)
				require.Zero(t, res.ExitCode, res.Stderr)
				require.Equal(t, fmt.Sprintf("oci://%s/%s/%s:%s\n", address, path, chart, registryTag), res.Stdout)

				pulled, pullRes := pull(t, tr, address, path, chart)
				require.NotEmpty(t, pulled, pullRes.Stderr)

				sent, err := os.ReadFile(published(t, chart))
				require.NoError(t, err)
				got, err := os.ReadFile(pulled)
				require.NoError(t, err)
				require.Equal(t, sent, got, "%s must come back byte for byte", chart)
			}
		})
	}
}

func TestPushAfterPackage(t *testing.T) {
	tr := harness.Transports()[0]
	s := harness.NewSession(t)
	address, path := s.LoginVia(t, tr), repoPath()

	packaged := harness.Cargueiro(t, t.TempDir(), "package", harness.FleetSource(t, "fleet-crd"), "--output-dir", t.TempDir())
	require.Zero(t, packaged.ExitCode, packaged.Stderr)
	archive := strings.TrimSpace(packaged.Stdout)

	res := s.Push(t, address+"/"+path, archive)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, fmt.Sprintf("oci://%s/%s/fleet-crd:%s\n", address, path, registryTag), res.Stdout)
	pulled, pullRes := pull(t, tr, address, path, "fleet-crd")
	require.NotEmpty(t, pulled, pullRes.Stderr)
	require.Equal(t,
		harness.Manifest(t, published(t, "fleet-crd")),
		harness.Manifest(t, pulled),
		"package then push must give back the chart published in rancher/charts")
}

func TestPushPasswordFromStdin(t *testing.T) {
	tr := harness.Transports()[0]
	s := harness.NewSession(t)
	address, path := s.LoginVia(t, tr), repoPath()

	res := s.Run(t, harness.RegistryPassword+"\n", nil,
		"push", published(t, "fleet-crd"), "--registry", address+"/"+path,
		"--username", harness.RegistryUser, "--password-stdin")

	requireNoPassword(t, res)
	require.Zero(t, res.ExitCode, res.Stderr)
	pulled, pullRes := pull(t, tr, address, path, "fleet-crd")
	require.NotEmpty(t, pulled, pullRes.Stderr)
}

func TestPushWithAWrongPassword(t *testing.T) {
	for _, tr := range harness.Transports() {
		t.Run(tr.Name, func(t *testing.T) {
			s := harness.NewSession(t)
			address, path := s.LoginVia(t, tr), repoPath()

			res := s.Run(t, "", []string{"CARGUEIRO_PASSWORD=not-the-password"},
				"push", published(t, "fleet-crd"), "--registry", address+"/"+path, "--username", harness.RegistryUser)

			require.NotZero(t, res.ExitCode, "the explicit wrong password wins over the saved right one")
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, "401")
			require.NotContains(t, res.Stderr, "not-the-password")
			pulled, _ := pull(t, tr, address, path, "fleet-crd")
			require.Empty(t, pulled, "nothing may have been stored")
		})
	}
}

func TestPushWithoutLoginCannotReachAnyRegistry(t *testing.T) {
	archive := published(t, "fleet-crd")

	tests := []struct {
		name    string
		address func(t *testing.T) string
		wantErr string
	}{
		{
			name:    "TLS registry with a private CA",
			address: harness.TLSRegistry,
			wantErr: "certificate",
		},
		{
			name:    "plain HTTP registry",
			address: harness.HTTPRegistry,
			wantErr: "HTTP response to HTTPS client",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := harness.NewSession(t)
			address := tt.address(t)

			res := s.Push(t, address+"/"+repoPath(), archive)

			require.NotZero(t, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
		})
	}
}

func TestPushCorruptArchive(t *testing.T) {
	s := harness.NewSession(t)
	address, path := s.LoginVia(t, harness.Transports()[0]), repoPath()
	corrupt := filepath.Join(t.TempDir(), "corrupt.tgz")
	require.NoError(t, os.WriteFile(corrupt, []byte("this is not a chart"), 0o600))

	res := s.Push(t, address+"/"+path, corrupt)

	require.NotZero(t, res.ExitCode)
	require.Empty(t, res.Stdout)
	require.Contains(t, res.Stderr, corrupt)
}

func TestPushBadInput(t *testing.T) {
	address := harness.HTTPRegistry(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no registry flag", args: []string{"push", published(t, "fleet-crd")}, wantErr: "registry"},
		{name: "no archive", args: []string{"push", "--registry", address}, wantErr: "accepts 1 arg(s), received 0"},
		{name: "two archives", args: []string{"push", published(t, "fleet-crd"), published(t, "fleet-agent"), "--registry", address}, wantErr: "accepts 1 arg(s), received 2"},
		{name: "invalid registry", args: []string{"push", published(t, "fleet-crd"), "--registry", address + "/a//b"}, wantErr: "invalid registry"},
		{name: "missing archive file", args: []string{"push", filepath.Join(t.TempDir(), "absent.tgz"), "--registry", address}, wantErr: "absent.tgz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := harness.Cargueiro(t, t.TempDir(), tt.args...)

			require.NotZero(t, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
		})
	}
}

// deadProxy makes any outbound HTTP(S) request fail locally. The protected
// registries are real SUSE hosts, so a regression in the guard must never be
// able to reach them.
func deadProxy() []string {
	const proxy = "http://127.0.0.1:1"
	return []string{
		"HTTP_PROXY=" + proxy, "HTTPS_PROXY=" + proxy, "ALL_PROXY=" + proxy,
		"http_proxy=" + proxy, "https_proxy=" + proxy, "all_proxy=" + proxy,
		"NO_PROXY=", "no_proxy=",
	}
}

func TestPushRefusesWhatTheSUSERegistriesForbid(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		chart    string
		wantErr  string
	}{
		{
			name:     "rancher chart in the prime repository",
			registry: "registry.suse.com/rancher/charts",
			chart:    "fleet-crd",
			wantErr:  "fleet-crd is a rancher-charts chart and cannot go to registry.suse.com/rancher/charts",
		},
		{
			name:     "rancher chart below the catalog",
			registry: "oci://stgregistry.suse.com/rancher/catalog/fleet",
			chart:    "fleet-crd",
			wantErr:  "fleet-crd is a rancher-charts chart and can only go to stgregistry.suse.com/rancher/catalog",
		},
		{
			name:     "capitals and port do not get around the edition",
			registry: "REGISTRY.suse.com:443/Rancher/Charts/",
			chart:    "fleet-crd",
			wantErr:  "fleet-crd is a rancher-charts chart and cannot go to",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := harness.CargueiroWith(t, harness.Options{
				Dir: t.TempDir(),
				Env: append(deadProxy(), "CARGUEIRO_PASSWORD="+harness.RegistryPassword),
			}, "push", published(t, tt.chart), "--registry", tt.registry, "--username", harness.RegistryUser)

			requireNoPassword(t, res)
			require.NotZero(t, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, "pushing to this registry is not allowed: "+tt.wantErr)
			require.NotContains(t, res.Stderr, "127.0.0.1:1", "the guard must act before any network attempt")
		})
	}
}

func TestPushMayOverwriteOnLocalRegistries(t *testing.T) {
	for _, tr := range harness.Transports() {
		t.Run(tr.Name, func(t *testing.T) {
			s := harness.NewSession(t)
			address, path := s.LoginVia(t, tr), repoPath()+"/rancher/catalog"
			archive := published(t, "fleet-crd")

			first := s.Push(t, address+"/"+path, archive)
			second := s.Push(t, address+"/"+path, archive)

			require.Zero(t, first.ExitCode, first.Stderr)
			require.Zero(t, second.ExitCode, "a local registry may be overwritten: "+second.Stderr)
			require.Equal(t, first.Stdout, second.Stdout)
		})
	}
}
