//go:build e2e

// Package pushing holds the e2e tests of `cargueiro push`, run against the real
// binary and real local registries (see `make registry-up`).
package pushing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
)

const fleetVersion = "110.0.3+up0.16.3"

// registryTag is how the registry stores fleetVersion: Helm turns "+" into "_".
const registryTag = "110.0.3_up0.16.3"

func fleetCharts() []string { return []string{"fleet-crd", "fleet", "fleet-agent"} }

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// transport describes one way of reaching a registry.
type transport struct {
	name    string
	address func(t *testing.T) string
	flags   func(t *testing.T) []string
}

func transports() []transport {
	return []transport{
		{
			name:    "HTTP",
			address: harness.HTTPRegistry,
			flags:   func(*testing.T) []string { return []string{"--plain-http"} },
		},
		{
			name:    "TLS",
			address: harness.TLSRegistry,
			flags:   func(t *testing.T) []string { return []string{"--ca-file", harness.RegistryCA(t)} },
		},
	}
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

// push runs `cargueiro push` as the e2e registry user and checks that the
// password never reaches the output.
func push(t *testing.T, registry string, flags []string, archives ...string) harness.Result {
	t.Helper()

	args := append([]string{"push"}, archives...)
	args = append(args, "--registry", registry, "--username", harness.RegistryUser)
	args = append(args, flags...)

	res := harness.CargueiroWith(t, harness.Options{
		Dir: t.TempDir(),
		Env: []string{"CARGUEIRO_PASSWORD=" + harness.RegistryPassword},
	}, args...)

	requireNoPassword(t, res)
	return res
}

func requireNoPassword(t *testing.T, res harness.Result) {
	t.Helper()

	require.NotContains(t, res.Stdout, harness.RegistryPassword)
	require.NotContains(t, res.Stderr, harness.RegistryPassword)
}

// pull fetches a chart from a registry with the helm CLI, which is the
// independent check that a push really stored something usable. It returns the
// path of the pulled archive, or "" and the failed result.
func pull(t *testing.T, address, path, chart string, flags []string) (string, harness.Result) {
	t.Helper()

	dest, cfg := t.TempDir(), t.TempDir()
	args := []string{
		"pull", "oci://" + address + "/" + path + "/" + chart,
		"--version", fleetVersion, "--destination", dest,
		"--username", harness.RegistryUser, "--password", harness.RegistryPassword,
	}
	args = append(args, flags...)

	res := harness.HelmWith(t, harness.Options{
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
	return filepath.Join(dest, chart+"-"+fleetVersion+".tgz"), res
}

// requireNothingStored fails if any fleet chart can be pulled from path.
func requireNothingStored(t *testing.T, tr transport, path string) {
	t.Helper()

	for _, chart := range fleetCharts() {
		archive, _ := pull(t, tr.address(t), path, chart, tr.flags(t))
		require.Empty(t, archive, "%s must not be in the registry", chart)
	}
}

func TestPushFleetFamily(t *testing.T) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			address, path := tr.address(t), repoPath()
			var archives []string
			for _, chart := range fleetCharts() {
				archives = append(archives, published(t, chart))
			}

			res := push(t, address+"/"+path, tr.flags(t), archives...)

			require.Zero(t, res.ExitCode, res.Stderr)
			var want []string
			for _, chart := range fleetCharts() {
				want = append(want, fmt.Sprintf("oci://%s/%s/%s:%s", address, path, chart, registryTag))
			}
			require.Equal(t, strings.Join(want, "\n")+"\n", res.Stdout, "references in argument order")

			for _, chart := range fleetCharts() {
				pulled, pullRes := pull(t, address, path, chart, tr.flags(t))
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
	address, path := harness.HTTPRegistry(t), repoPath()

	packaged := harness.Cargueiro(t, t.TempDir(), "package", harness.FleetSource(t, "fleet-crd"), "--output-dir", t.TempDir())
	require.Zero(t, packaged.ExitCode, packaged.Stderr)
	archive := strings.TrimSpace(packaged.Stdout)

	res := push(t, address+"/"+path, []string{"--plain-http"}, archive)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, fmt.Sprintf("oci://%s/%s/fleet-crd:%s\n", address, path, registryTag), res.Stdout)

	pulled, pullRes := pull(t, address, path, "fleet-crd", []string{"--plain-http"})
	require.NotEmpty(t, pulled, pullRes.Stderr)
	require.Equal(t,
		harness.Manifest(t, published(t, "fleet-crd")),
		harness.Manifest(t, pulled),
		"package then push must give back the chart published in rancher/charts")
}

func TestPushPasswordFromStdin(t *testing.T) {
	address, path := harness.HTTPRegistry(t), repoPath()

	res := harness.CargueiroWith(t, harness.Options{
		Dir:   t.TempDir(),
		Stdin: harness.RegistryPassword + "\n",
	}, "push", published(t, "fleet-crd"),
		"--registry", address+"/"+path, "--username", harness.RegistryUser, "--password-stdin", "--plain-http")

	requireNoPassword(t, res)
	require.Zero(t, res.ExitCode, res.Stderr)
	pulled, pullRes := pull(t, address, path, "fleet-crd", []string{"--plain-http"})
	require.NotEmpty(t, pulled, pullRes.Stderr)
}

func TestPushAuthenticationFailures(t *testing.T) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			address, path := tr.address(t), repoPath()
			archive := published(t, "fleet-crd")

			t.Run("wrong password", func(t *testing.T) {
				res := harness.CargueiroWith(t, harness.Options{
					Dir: t.TempDir(),
					Env: []string{"CARGUEIRO_PASSWORD=not-the-password"},
				}, append([]string{"push", archive, "--registry", address + "/" + path, "--username", harness.RegistryUser}, tr.flags(t)...)...)

				require.NotZero(t, res.ExitCode)
				require.Empty(t, res.Stdout)
				require.Contains(t, res.Stderr, "401")
				require.NotContains(t, res.Stderr, "not-the-password")
			})

			t.Run("no credentials", func(t *testing.T) {
				res := harness.Cargueiro(t, t.TempDir(),
					append([]string{"push", archive, "--registry", address + "/" + path}, tr.flags(t)...)...)

				require.NotZero(t, res.ExitCode)
				require.Empty(t, res.Stdout)
				require.Regexp(t, `401|credential`, res.Stderr, "the registry demands credentials")
			})

			t.Run("nothing was stored", func(t *testing.T) {
				requireNothingStored(t, tr, path)
			})
		})
	}
}

func TestPushTransportMismatches(t *testing.T) {
	archive := published(t, "fleet-crd")

	tests := []struct {
		name    string
		address func(t *testing.T) string
		flags   func(t *testing.T) []string
		wantErr string
	}{
		{
			name:    "TLS registry without its CA",
			address: harness.TLSRegistry,
			flags:   func(*testing.T) []string { return nil },
			wantErr: "certificate",
		},
		{
			name:    "plain HTTP against the TLS registry",
			address: harness.TLSRegistry,
			flags:   func(*testing.T) []string { return []string{"--plain-http"} },
			wantErr: "400",
		},
		{
			name:    "HTTPS against the plain HTTP registry",
			address: harness.HTTPRegistry,
			flags:   func(*testing.T) []string { return nil },
			wantErr: "HTTP response to HTTPS client",
		},
		{
			name:    "CA file together with plain HTTP",
			address: harness.TLSRegistry,
			flags:   func(t *testing.T) []string { return []string{"--plain-http", "--ca-file", harness.RegistryCA(t)} },
			wantErr: "plain HTTP",
		},
		{
			name:    "CA file that does not exist",
			address: harness.TLSRegistry,
			flags:   func(t *testing.T) []string { return []string{"--ca-file", filepath.Join(t.TempDir(), "absent.crt")} },
			wantErr: "read CA file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, path := tt.address(t), repoPath()

			res := push(t, address+"/"+path, tt.flags(t), archive)

			require.NotZero(t, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
		})
	}
}

func TestPushPartialFailure(t *testing.T) {
	address, path := harness.HTTPRegistry(t), repoPath()
	corrupt := filepath.Join(t.TempDir(), "corrupt.tgz")
	require.NoError(t, os.WriteFile(corrupt, []byte("this is not a chart"), 0o600))

	res := push(t, address+"/"+path, []string{"--plain-http"},
		published(t, "fleet-crd"), corrupt, published(t, "fleet-agent"))

	require.NotZero(t, res.ExitCode)
	require.Equal(t,
		fmt.Sprintf("oci://%s/%s/fleet-crd:%s\noci://%s/%s/fleet-agent:%s\n", address, path, registryTag, address, path, registryTag),
		res.Stdout, "the good archives are still pushed")
	require.Contains(t, res.Stderr, "1 of 3 archives failed")
	require.Contains(t, res.Stderr, corrupt)

	for _, chart := range []string{"fleet-crd", "fleet-agent"} {
		pulled, pullRes := pull(t, address, path, chart, []string{"--plain-http"})
		require.NotEmpty(t, pulled, pullRes.Stderr)
	}
}

func TestPushBadInput(t *testing.T) {
	address := harness.HTTPRegistry(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no registry flag", args: []string{"push", published(t, "fleet-crd")}, wantErr: "registry"},
		{name: "no archive", args: []string{"push", "--registry", address}, wantErr: "requires at least 1 arg"},
		{name: "invalid registry", args: []string{"push", published(t, "fleet-crd"), "--registry", address + "/a//b"}, wantErr: "invalid registry"},
		{name: "missing archive file", args: []string{"push", filepath.Join(t.TempDir(), "absent.tgz"), "--registry", address, "--plain-http"}, wantErr: "absent.tgz"},
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
