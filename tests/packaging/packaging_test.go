//go:build e2e

// Package packaging holds the e2e tests of `cargueiro package`, run against the
// real binary and real SUSE charts.
package packaging

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

const fleetVersion = "110.0.3+up0.16.3"

func fleetCharts() []string { return []string{"fleet-crd", "fleet", "fleet-agent"} }

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// packageOne packages a fleet chart with cargueiro and returns the archive path.
func packageOne(t *testing.T, chart, outDir string) string {
	t.Helper()

	res := harness.Cargueiro(t, t.TempDir(), "package", harness.FleetSource(t, chart), "--output-dir", outDir)
	require.Zero(t, res.ExitCode, res.Stderr)

	archive := strings.TrimSpace(res.Stdout)
	require.Equal(t, filepath.Join(outDir, chart+"-"+fleetVersion+".tgz"), archive)
	return archive
}

// helmPackage packages a fleet chart with the helm CLI and returns the archive path.
func helmPackage(t *testing.T, chart, outDir string) string {
	t.Helper()

	res := harness.Helm(t, t.TempDir(), "package", harness.FleetSource(t, chart), "--destination", outDir)
	require.Zero(t, res.ExitCode, res.Stderr)

	return filepath.Join(outDir, chart+"-"+fleetVersion+".tgz")
}

func TestHelmCLIMatchesSDK(t *testing.T) {
	harness.RequireHelmMatchesSDK(t)
}

// TestFleetThreeWay compares, per chart, the archive built by cargueiro, by the
// helm CLI and the one published in rancher/charts. Each pair is its own
// subtest so a failure says which two sources disagree.
func TestFleetThreeWay(t *testing.T) {
	harness.RequireHelmMatchesSDK(t)

	for _, chart := range fleetCharts() {
		t.Run(chart, func(t *testing.T) {
			ours := harness.Manifest(t, packageOne(t, chart, t.TempDir()))
			helm := harness.Manifest(t, helmPackage(t, chart, t.TempDir()))
			published := harness.Manifest(t, harness.FleetPublished(t, chart, fleetVersion))

			t.Run("cargueiro equals helm CLI", func(t *testing.T) {
				require.Equal(t, helm, ours)
			})
			t.Run("cargueiro equals published", func(t *testing.T) {
				require.Equal(t, published, ours)
			})
			t.Run("helm CLI equals published", func(t *testing.T) {
				require.Equal(t, published, helm)
			})
		})
	}
}

func TestDefaultOutputDirIsAssets(t *testing.T) {
	workDir := t.TempDir()

	res := harness.Cargueiro(t, workDir, "package", harness.FleetSource(t, "fleet-crd"))

	require.Zero(t, res.ExitCode, res.Stderr)
	want := filepath.Join(workDir, "assets", "fleet-crd-"+fleetVersion+".tgz")
	require.Equal(t, want, strings.TrimSpace(res.Stdout))
	require.FileExists(t, want)
}

func TestPackagingDoesNotModifySource(t *testing.T) {
	for _, chart := range fleetCharts() {
		t.Run(chart, func(t *testing.T) {
			src := harness.FleetSource(t, chart)
			before := treeHash(t, src)

			packageOne(t, chart, t.TempDir())

			require.Equal(t, before, treeHash(t, src))
		})
	}
}

func TestPackagingIsDeterministic(t *testing.T) {
	first := harness.Manifest(t, packageOne(t, "fleet", t.TempDir()))
	second := harness.Manifest(t, packageOne(t, "fleet", t.TempDir()))

	require.Equal(t, first, second)
}

func TestFailures(t *testing.T) {
	notAChart := t.TempDir()

	t.Run("directory without Chart.yaml", func(t *testing.T) {
		res := harness.Cargueiro(t, t.TempDir(), "package", notAChart, "-o", t.TempDir())

		require.NotZero(t, res.ExitCode)
		require.Empty(t, res.Stdout)
		require.Contains(t, res.Stderr, "Chart.yaml not found")
	})

	t.Run("two charts", func(t *testing.T) {
		res := harness.Cargueiro(t, t.TempDir(), "package",
			harness.FleetSource(t, "fleet-crd"), harness.FleetSource(t, "fleet-agent"), "-o", t.TempDir())

		require.NotZero(t, res.ExitCode)
		require.Empty(t, res.Stdout)
		require.Contains(t, res.Stderr, "accepts 1 arg(s), received 2")
	})
}

// treeHash returns a digest of every file path, mode and content under dir.
func treeHash(t *testing.T, dir string) string {
	t.Helper()

	var entries []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		sum := ""
		if d.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum = fmt.Sprintf("%x", sha256.Sum256(data))
		}
		entries = append(entries, rel+"|"+info.Mode().String()+"|"+sum)
		return nil
	})
	require.NoError(t, err)

	sort.Strings(entries)
	return strings.Join(entries, "\n")
}
