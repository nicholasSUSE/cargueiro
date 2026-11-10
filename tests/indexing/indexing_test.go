//go:build e2e

// Package indexing holds the e2e tests of `cargueiro index`, run against the
// real binary and the charts repository kept in tests/data/rancher-charts. Each
// test works on a copy of it, in a temporary directory.
package indexing

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
)

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// fixture returns the path of the charts repository kept in tests/data.
func fixture(t *testing.T) string {
	t.Helper()

	return filepath.Join(harness.RepoRoot(t), "tests", "data", "rancher-charts")
}

// copyRepo copies the entries and the logos of the fixture to a new directory,
// without its index.yaml, and returns it.
func copyRepo(t *testing.T) string {
	t.Helper()

	src := fixture(t)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "index.yaml" {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	require.NoError(t, err)
	return dst
}

func readIndex(t *testing.T, repo string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repo, "index.yaml"))
	require.NoError(t, err)
	return string(data)
}

func TestIndexMatchesTheExpectedFile(t *testing.T) {
	repo := copyRepo(t)

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)
	require.Zero(t, res.ExitCode, res.Stderr)

	want, err := os.ReadFile(filepath.Join(fixture(t), "index.yaml"))
	require.NoError(t, err)
	require.Equal(t, string(want), readIndex(t, repo))
}

func TestIndexGivesTheSameBytesEveryTime(t *testing.T) {
	repo := copyRepo(t)

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)
	require.Zero(t, res.ExitCode, res.Stderr)
	first := readIndex(t, repo)

	res = harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)
	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, first, readIndex(t, repo))
}

func TestIndexRefusesTwoEntriesWithTheSameVersion(t *testing.T) {
	repo := copyRepo(t)
	copyOf, err := os.ReadFile(filepath.Join(repo, "entries", "fleet", "111.0.3+up0.17.3.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "entries", "fleet", "copy.yaml"), copyOf, 0o600))

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "111.0.3+up0.17.3")
	require.NoFileExists(t, filepath.Join(repo, "index.yaml"))
}

func TestIndexRefusesAnEntryNamedForAnotherVersion(t *testing.T) {
	repo := copyRepo(t)
	dir := filepath.Join(repo, "entries", "fleet")
	require.NoError(t, os.Rename(filepath.Join(dir, "111.0.3+up0.17.3.yaml"), filepath.Join(dir, "111.0.9+up0.17.9.yaml")))

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "111.0.9+up0.17.9")
	require.NoFileExists(t, filepath.Join(repo, "index.yaml"))
}
