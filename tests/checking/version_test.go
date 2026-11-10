//go:build e2e

package checking

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/check"
	"github.com/rancher/cargueiro/tests/internal/harness"
)

// withVersion copies Chart.yaml and values.yaml of a chart directory and
// changes the version of the copy. The chart is the real one; the version is
// the only difference.
func withVersion(t *testing.T, src, from, to string) string {
	t.Helper()

	dst := t.TempDir()
	for _, file := range []string{"Chart.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join(src, file))
		require.NoError(t, err)
		if file == "Chart.yaml" {
			changed := strings.Replace(string(data), "\nversion: "+from+"\n", "\nversion: "+to+"\n", 1)
			require.NotEqual(t, string(data), changed, "Chart.yaml has no version %s", from)
			data = []byte(changed)
		}
		require.NoError(t, os.WriteFile(filepath.Join(dst, file), data, 0o600))
	}
	return dst
}

// archiveOf packs Chart.yaml and values.yaml of a directory as a chart archive.
// A directory with no values.yaml gives an archive with none.
func archiveOf(t *testing.T, dir, name string) string {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, file := range []string{"Chart.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name + "/" + file,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(data)),
		}))
		_, err = tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	path := filepath.Join(t.TempDir(), name+".tgz")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

// decode reads the JSON result of a check command.
func decode(t *testing.T, res harness.Result) check.Result {
	t.Helper()

	var got check.Result
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got), "stdout: %s\nstderr: %s", res.Stdout, res.Stderr)
	return got
}

// requireOnlyTheVersionFails checks a result whose version is the one thing
// wrong, and its exit code.
func requireOnlyTheVersionFails(t *testing.T, res harness.Result, wantErr string) {
	t.Helper()

	require.Equal(t, 1, res.ExitCode)
	require.Contains(t, res.Stderr, "chart is not valid")
	got := decode(t, res)
	require.False(t, got.Valid)
	require.True(t, got.Annotations.Valid)
	require.True(t, got.HIP15Annotations.Valid)
	require.False(t, got.Version.Valid)
	require.Equal(t, []string{wantErr}, got.Version.Errors)
}

// The Rancher 2.15 fleet chart, with a version that is wrong in one way each.
const (
	wrongMajor = `version "109.0.3+up0.16.3" has major 109, but charts for Rancher 2.15 have major 110`
	noUp       = `version "110.0.3" must look like 1XY.Z.W+upA.B.C, optionally followed by -rancher.N, -rc.N, -beta.N or -alpha.N`
)

func TestCheckChartVersion(t *testing.T) {
	fleet := harness.FleetSource(t, "fleet")

	tests := []struct {
		name    string
		version string
		wantErr string
	}{
		{
			name:    "the major of Rancher 2.14, on a chart for Rancher 2.15",
			version: "109.0.3+up0.16.3",
			wantErr: wrongMajor,
		},
		{
			name:    "no +up part",
			version: "110.0.3",
			wantErr: noUp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := withVersion(t, fleet, fleetVersion, tt.version)

			res := checkChartText(t, dir, "--output", "json")

			requireOnlyTheVersionFails(t, res, tt.wantErr)
			require.Equal(t, tt.version, decode(t, res).Metadata.Version)
		})
	}

	t.Run("a prerelease is allowed", func(t *testing.T) {
		dir := withVersion(t, fleet, fleetVersion, "110.0.3+up0.16.3-rc.1")

		res := checkChartText(t, dir, "--output", "json")

		require.Zero(t, res.ExitCode, res.Stderr)
		require.True(t, decode(t, res).Version.Valid)
	})

	t.Run("--no-error-hip-15 does not ignore a version error", func(t *testing.T) {
		dir := withVersion(t, fleet, fleetVersion, "109.0.3+up0.16.3")

		res := checkChartText(t, dir, "--output", "json", "--no-error-hip-15")

		requireOnlyTheVersionFails(t, res, wrongMajor)
		require.NotContains(t, res.Stderr, "ignored")
	})

	t.Run("text output", func(t *testing.T) {
		dir := withVersion(t, fleet, fleetVersion, "109.0.3+up0.16.3")

		res := checkChartText(t, dir)

		require.Equal(t, 1, res.ExitCode)
		require.Contains(t, res.Stdout, "version: invalid\n  - "+wrongMajor+"\nchart: fleet\n")
	})
}

func TestCheckChartVersionOfACRD(t *testing.T) {
	crd := harness.FleetSource(t, "fleet-crd")

	t.Run("a CRD has no rancher-version, so only its form is checked", func(t *testing.T) {
		dir := withVersion(t, crd, fleetVersion, "109.0.3+up0.16.3")

		res := checkChartText(t, dir, "--output", "json")

		require.Zero(t, res.ExitCode, res.Stderr)
		require.True(t, decode(t, res).Version.Valid)
	})

	t.Run("a CRD with a bad form", func(t *testing.T) {
		dir := withVersion(t, crd, fleetVersion, "110.0.3")

		res := checkChartText(t, dir, "--output", "json")

		requireOnlyTheVersionFails(t, res, noUp)
	})
}

func TestCheckChartVersionOfAPlainChart(t *testing.T) {
	suc := harness.ChartSource(t, "system-upgrade-controller", "system-upgrade-controller")

	dir := withVersion(t, suc, "110.0.0", "110.0.0+up0.20.1")
	res := checkChartText(t, dir, "--output", "json", "--no-error-hip-15")

	require.Equal(t, 1, res.ExitCode)
	got := decode(t, res)
	require.False(t, got.Version.Valid)
	require.Equal(t, []string{`version "110.0.0+up0.20.1" must look like 1XY.Z.W`}, got.Version.Errors)
}

func TestCheckArchiveVersion(t *testing.T) {
	dir := withVersion(t, harness.FleetSource(t, "fleet"), fleetVersion, "109.0.3+up0.16.3")

	res := checkArchive(t, archiveOf(t, dir, "fleet"), "--output", "json")

	requireOnlyTheVersionFails(t, res, wrongMajor)

	// The same chart gives the same report from an archive and from a directory.
	fromArchive := checkArchive(t, archiveOf(t, dir, "fleet"))
	fromDirectory := checkChartText(t, dir)
	require.Equal(t, fromDirectory.Stdout, fromArchive.Stdout)
}

func TestCheckOCIVersion(t *testing.T) {
	address, session := openRegistry(t)
	dir := withVersion(t, harness.FleetSource(t, "fleet"), fleetVersion, "109.0.3+up0.16.3")
	pushArchive(t, session, address, archiveOf(t, dir, "fleet"))

	t.Run("a version with a plus", func(t *testing.T) {
		res := checkOCI(t, address+"/rancher/fleet:109.0.3+up0.16.3", "--plain-http", "--output", "json")

		requireOnlyTheVersionFails(t, res, wrongMajor)
	})

	t.Run("the tag as the registry stores it", func(t *testing.T) {
		res := checkOCI(t, address+"/rancher/fleet:109.0.3_up0.16.3", "--plain-http", "--output", "json")

		requireOnlyTheVersionFails(t, res, wrongMajor)
	})

	t.Run("the same chart from a directory", func(t *testing.T) {
		fromRegistry := checkOCI(t, address+"/rancher/fleet:109.0.3+up0.16.3", "--plain-http")
		fromDirectory := checkChartText(t, dir)

		require.Equal(t, fromDirectory.Stdout, fromRegistry.Stdout)
	})
}
