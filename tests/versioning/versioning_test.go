//go:build e2e

// Package versioning holds the e2e tests of `cargueiro chart-version`, run
// against the real binary, the charts kept in tests/data and the local registry
// that asks for no password (see `make registry-up`). Each test works on a copy
// of a chart, in a temporary directory.
package versioning

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

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// copyChart copies Chart.yaml and values.yaml (when the fixture has one) of a
// chart kept in tests/data to a new directory, and returns it.
func copyChart(t *testing.T, dataDir, name string) string {
	t.Helper()

	src := harness.ChartSource(t, dataDir, name)
	dst := t.TempDir()
	for _, file := range []string{"Chart.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join(src, file))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dst, file), data, 0o600))
	}
	return dst
}

// edit changes a file of the chart directory, which must contain old once.
func edit(t *testing.T, dir, file, old, replacement string) {
	t.Helper()

	path := filepath.Join(dir, file)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(data), old), "%s must contain %q once", file, old)
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), old, replacement, 1)), 0o600))
}

func readChartYAML(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	return string(data)
}

func chartVersion(t *testing.T, dir string, args ...string) harness.Result {
	t.Helper()

	return harness.Cargueiro(t, t.TempDir(), append([]string{"chart-version", dir}, args...)...)
}

// deadProxy makes any outbound HTTP(S) request fail locally. Without a registry
// given, chart-version reads the staging registry of SUSE, so no test that
// leaves the registry out may reach it.
func deadProxy() []string {
	const proxy = "http://127.0.0.1:1"
	return []string{
		"HTTP_PROXY=" + proxy, "HTTPS_PROXY=" + proxy, "ALL_PROXY=" + proxy,
		"http_proxy=" + proxy, "https_proxy=" + proxy, "all_proxy=" + proxy,
		"NO_PROXY=", "no_proxy=",
	}
}

// TestChartVersionFollowsTheReleases follows a chart through its releases
// against the local registry: a first release, two candidates and the final of
// the next upstream version. Each version is worked out, stamped, packaged and
// pushed before the next is worked out, and list then puts them in order.
func TestChartVersionFollowsTheReleases(t *testing.T) {
	address := harness.OpenRegistry(t)
	session := harness.NewSession(t)
	login := session.Login(t, address, []string{"--plain-http"})
	require.Zero(t, login.ExitCode, login.Stderr)
	path := fmt.Sprintf("e2e/%d", time.Now().UnixNano())
	registry := []string{"--registry", address + "/" + path, "--plain-http"}

	// release works out the version of a copy of the chart with the appVersion,
	// expecting want, stamps it with that version and pushes it.
	release := func(t *testing.T, appVersion, want string) {
		t.Helper()

		dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
		edit(t, dir, "Chart.yaml", "appVersion: v1.5.4", "appVersion: "+appVersion)

		res := chartVersion(t, dir, registry...)
		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, want+"\n", res.Stdout)

		stamp := harness.Cargueiro(t, t.TempDir(), "stamp", dir, "--version", strings.TrimSpace(res.Stdout))
		require.Zero(t, stamp.ExitCode, stamp.Stderr)
		pkg := harness.Cargueiro(t, t.TempDir(), "package", dir, "--output-dir", t.TempDir())
		require.Zero(t, pkg.ExitCode, pkg.Stderr)
		push := session.Push(t, address+"/"+path, strings.TrimSpace(pkg.Stdout))
		require.Zero(t, push.ExitCode, push.Stderr)
	}

	release(t, "v1.5.4", "111.0.0+up1.5.4")
	release(t, "v1.5.5-rc.1", "111.0.1+up1.5.5-rc.1")
	release(t, "v1.5.5-rc.2", "111.0.1+up1.5.5-rc.2")
	release(t, "v1.5.5", "111.0.1+up1.5.5")

	t.Run("a candidate after the final of its upstream version takes the next counter", func(t *testing.T) {
		dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
		edit(t, dir, "Chart.yaml", "appVersion: v1.5.4", "appVersion: v1.5.5-rc.1")

		res := chartVersion(t, dir, registry...)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "111.0.2+up1.5.5-rc.1\n", res.Stdout)
	})

	t.Run("the next upstream version takes the next counter", func(t *testing.T) {
		dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
		edit(t, dir, "Chart.yaml", "appVersion: v1.5.4", "appVersion: v1.5.6")

		res := chartVersion(t, dir, registry...)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "111.0.2+up1.5.6\n", res.Stdout)
	})

	t.Run("list puts the releases in order", func(t *testing.T) {
		res := harness.Cargueiro(t, t.TempDir(), "list", address+"/"+path+"/rancher-compliance", "--plain-http")

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "111.0.1+up1.5.5\n111.0.1+up1.5.5-rc.2\n111.0.1+up1.5.5-rc.1\n111.0.0+up1.5.4\n", res.Stdout)
	})

	t.Run("a chart that was never pushed starts at the first version", func(t *testing.T) {
		dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")

		res := chartVersion(t, dir, "--registry", address+"/"+path+"-never-pushed", "--plain-http")

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "111.0.0+up1.5.4\n", res.Stdout)
	})

	t.Run("it writes nothing", func(t *testing.T) {
		dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
		before := readChartYAML(t, dir)

		res := chartVersion(t, dir, registry...)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, before, readChartYAML(t, dir))
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 2, "and leaves no file")
	})
}

func TestChartVersionReadsTheStagingRegistryOfTheEdition(t *testing.T) {
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")

	res := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: deadProxy()}, "chart-version", dir)

	require.Equal(t, 1, res.ExitCode)
	require.Empty(t, res.Stdout)
	require.Contains(t, res.Stderr, "stgregistry.suse.com/rancher/catalog/rancher-compliance")
}

func TestChartVersionCannotWorkOutTheVersion(t *testing.T) {
	tests := []struct {
		name    string
		chart   func(t *testing.T) string
		wantErr string
	}{
		{
			name: "a CRD chart has no rancher-version",
			chart: func(t *testing.T) string {
				return copyChart(t, "fleet", "fleet-crd")
			},
			wantErr: "a CRD chart has no rancher-version, so its version cannot be worked out on its own",
		},
		{
			name: "no appVersion",
			chart: func(t *testing.T) string {
				dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
				edit(t, dir, "Chart.yaml", "appVersion: v1.5.4\n", "")
				return dir
			},
			wantErr: "the Chart.yaml has no appVersion",
		},
		{
			name: "a chart for Rancher 2.15",
			chart: func(t *testing.T) string {
				dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
				edit(t, dir, "Chart.yaml", ">= 2.16.0-0 < 2.17.0-0", ">= 2.15.0-0 < 2.16.0-0")
				return dir
			},
			wantErr: "2.15 and before are not",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.chart(t)
			before := readChartYAML(t, dir)

			res := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: deadProxy()}, "chart-version", dir)

			require.Equal(t, 1, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
			require.Equal(t, before, readChartYAML(t, dir))
		})
	}
}
