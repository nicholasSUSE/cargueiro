//go:build e2e

// Package stamping holds the e2e tests of `cargueiro stamp`, run against the real
// binary and the charts kept in tests/data. Each test works on a copy of a chart,
// in a temporary directory.
package stamping

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/check"
	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
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

func stamp(t *testing.T, dir string, args ...string) harness.Result {
	t.Helper()

	return harness.Cargueiro(t, t.TempDir(), append([]string{"stamp", dir}, args...)...)
}

// checkChart runs check chart on a directory and decodes its JSON result.
func checkChart(t *testing.T, dir string) (check.Result, harness.Result) {
	t.Helper()

	res := harness.Cargueiro(t, t.TempDir(), "check", "chart", dir, "--output", "json")
	var got check.Result
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got), "stdout: %s\nstderr: %s", res.Stdout, res.Stderr)
	return got, res
}

// TestStampThenCheckAndPackage stamps the compliance chart as its upstream keeps
// it (real image tags, the upstream version), and follows it through what a
// chart owner does next: check the directory, package it, check the archive.
func TestStampThenCheckAndPackage(t *testing.T) {
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
	before := readChartYAML(t, dir)

	res := stamp(t, dir, "--version", "111.0.0+up1.5.4")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Empty(t, res.Stderr)
	require.Equal(t, "version: 1.5.4 -> 111.0.0+up1.5.4\n"+
		"helm.sh/images: added kuberlr-kubectl, compliance-operator, security-scan, mirrored-sonobuoy-sonobuoy\n", res.Stdout)

	after := readChartYAML(t, dir)
	// The only changes are the version line and the new block, added at the end
	// of the annotations.
	const lastAnnotation = "  catalog.cattle.io/ui-component: rancher-compliance\n"
	const block = "  helm.sh/images: |\n" +
		"    - name: kuberlr-kubectl\n      image: rancher/kuberlr-kubectl:v8.1.2\n" +
		"    - name: compliance-operator\n      image: rancher/compliance-operator:v1.5.4\n" +
		"    - name: security-scan\n      image: rancher/security-scan:v0.10.4\n" +
		"    - name: mirrored-sonobuoy-sonobuoy\n      image: rancher/mirrored-sonobuoy-sonobuoy:v0.57.5\n"
	want := strings.Replace(before, "\nversion: 1.5.4\n", "\nversion: 111.0.0+up1.5.4\n", 1)
	want = strings.Replace(want, lastAnnotation, lastAnnotation+block, 1)
	require.Equal(t, want, after)

	t.Run("check chart", func(t *testing.T) {
		got, res := checkChart(t, dir)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.True(t, got.Valid, "%+v", got)
		require.Equal(t, "111.0.0+up1.5.4", got.Metadata.Version)
	})

	t.Run("package, then check archive", func(t *testing.T) {
		pkg := harness.Cargueiro(t, t.TempDir(), "package", dir, "--output-dir", t.TempDir())
		require.Zero(t, pkg.ExitCode, pkg.Stderr)
		archive := strings.TrimSpace(pkg.Stdout)
		require.True(t, strings.HasSuffix(archive, "rancher-compliance-111.0.0+up1.5.4.tgz"), archive)

		res := harness.Cargueiro(t, t.TempDir(), "check", "archive", archive)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.True(t, strings.HasPrefix(res.Stdout, "valid: true\n"), res.Stdout)
	})

	t.Run("a second time nothing changes", func(t *testing.T) {
		again := readChartYAML(t, dir)

		res := stamp(t, dir, "--version", "111.0.0+up1.5.4")

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "nothing to change\n", res.Stdout)
		require.Equal(t, again, readChartYAML(t, dir))
	})
}

func TestStampTheWebhookChart(t *testing.T) {
	t.Run("as upstream keeps it, the tag latest is refused and nothing changes", func(t *testing.T) {
		dir := copyChart(t, "rancher-webhook-upstream", "rancher-webhook")
		before := readChartYAML(t, dir)

		res := stamp(t, dir, "--version", "111.0.4+up0.11.4")

		require.Equal(t, 1, res.ExitCode)
		require.Empty(t, res.Stdout)
		require.Contains(t, res.Stderr, `values.yaml image: "tag" must not be "latest"`)
		require.Equal(t, before, readChartYAML(t, dir))
	})

	t.Run("with the tag the owner's automation sets", func(t *testing.T) {
		dir := copyChart(t, "rancher-webhook-upstream", "rancher-webhook")
		edit(t, dir, "values.yaml", "tag: latest", "tag: v0.11.4")

		res := stamp(t, dir, "--version", "111.0.4+up0.11.4")

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "version: 999 -> 111.0.4+up0.11.4\nhelm.sh/images: added rancher-webhook\n", res.Stdout)

		got, _ := checkChart(t, dir)
		require.Equal(t, check.Section{Valid: true, Errors: []string{}}, got.HIP15Annotations)
		require.Equal(t, check.Section{Valid: true, Errors: []string{}}, got.Version)
	})
}

func TestStampRefusesAndChangesNothing(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, dir string)
		version string
		wantErr string
	}{
		{
			name:    "a version without a +up part",
			version: "111.0.0",
			wantErr: `version "111.0.0" must look like 1XY.Z.W+upA.B.C`,
		},
		{
			name:    "a prerelease name that is not allowed",
			version: "111.0.0+up1.5.4-dev.1",
			wantErr: `version "111.0.0+up1.5.4-dev.1" must look like`,
		},
		{
			name:    "the major of the old system",
			version: "110.3.0+up1.5.4",
			wantErr: "has major 110, but charts for Rancher 2.16 have major 111",
		},
		{
			name: "a chart for Rancher 2.15",
			prepare: func(t *testing.T, dir string) {
				edit(t, dir, "Chart.yaml", ">= 2.16.0-0 < 2.17.0-0", ">= 2.15.0-0 < 2.16.0-0")
			},
			version: "110.3.0+up1.5.4",
			wantErr: "Rancher 2.16 and later are supported, 2.15 and before are not",
		},
		{
			name: "no rancher-version",
			prepare: func(t *testing.T, dir string) {
				edit(t, dir, "Chart.yaml", "  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'\n", "")
			},
			version: "111.0.0+up1.5.4",
			wantErr: "has no lower bound",
		},
		{
			name: "an image without a tag",
			prepare: func(t *testing.T, dir string) {
				edit(t, dir, "values.yaml", "    tag: v1.5.4\n", "")
			},
			version: "111.0.0+up1.5.4",
			wantErr: `"repository" is set but "tag" is empty`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
			if tt.prepare != nil {
				tt.prepare(t, dir)
			}
			before := readChartYAML(t, dir)

			res := stamp(t, dir, "--version", tt.version)

			require.Equal(t, 1, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
			require.Equal(t, before, readChartYAML(t, dir), "nothing is written when a check fails")
		})
	}
}

func TestStampAChartWithoutValues(t *testing.T) {
	dir := copyChart(t, "fleet", "fleet-crd")
	require.NoError(t, os.Remove(filepath.Join(dir, "values.yaml")))

	res := stamp(t, dir, "--version", "111.0.0+up0.16.3")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "version: 110.0.3+up0.16.3 -> 111.0.0+up0.16.3\n", res.Stdout)
	require.Contains(t, readChartYAML(t, dir), "\nversion: 111.0.0+up0.16.3\n")
}

func TestStampNeedsAVersion(t *testing.T) {
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
	before := readChartYAML(t, dir)

	res := stamp(t, dir)

	require.Equal(t, 1, res.ExitCode)
	require.Empty(t, res.Stdout)
	require.Contains(t, res.Stderr, `required flag(s) "version" not set`)
	require.Equal(t, before, readChartYAML(t, dir))
}
