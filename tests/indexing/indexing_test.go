//go:build e2e

// Package indexing holds the e2e tests of `cargueiro index`, run against the
// real binary and the charts repository kept in tests/data/rancher-charts. Each
// test works on a copy of it, in a temporary directory.
package indexing

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// publishedRepo returns a copy of the fixture as a repository that has already
// been published: it keeps the index.yaml, and every entry has an archive. The
// archives are empty files, because only their names are looked at.
func publishedRepo(t *testing.T) string {
	t.Helper()

	repo := copyRepo(t)
	want, err := os.ReadFile(filepath.Join(fixture(t), "index.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "index.yaml"), want, 0o600))

	addArchives(t, repo)
	return repo
}

// addArchives adds an empty archive for every entry of the repository.
func addArchives(t *testing.T, repo string) {
	t.Helper()

	entries, err := filepath.Glob(filepath.Join(repo, "entries", "*", "*.yaml"))
	require.NoError(t, err)
	for _, entry := range entries {
		addArchive(t, repo, filepath.Base(filepath.Dir(entry)), strings.TrimSuffix(filepath.Base(entry), ".yaml"))
	}
}

// addArchive adds an empty archive of a version of a chart to assets/.
func addArchive(t *testing.T, repo, chart, version string) {
	t.Helper()

	dir := filepath.Join(repo, "assets", chart)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, chart+"-"+version+".tgz"), nil, 0o600))
}

// hideEntry marks the entry of a version of a chart as hidden.
func hideEntry(t *testing.T, repo, chart, version string) {
	t.Helper()

	path := filepath.Join(repo, "entries", chart, version+".yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(data, "hide: true\n"...), 0o600))
}

// unhideEntry takes the hide mark off the entry of a version of a chart.
func unhideEntry(t *testing.T, repo, chart, version string) {
	t.Helper()

	path := filepath.Join(repo, "entries", chart, version+".yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "hide: true\n")
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), "hide: true\n", "", 1)), 0o600))
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
	addArchives(t, repo) // what pulling the new versions will do

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

func TestIndexLeavesAPublishedRepositoryAsItIs(t *testing.T) {
	repo := publishedRepo(t)
	before := readIndex(t, repo)

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, before, readIndex(t, repo))
}

func TestIndexRefusesWhenTheThreePlacesDisagree(t *testing.T) {
	tests := []struct {
		name    string
		repo    func(t *testing.T) string
		change  func(t *testing.T, repo string)
		wantErr string
	}{
		{
			name: "a published version lost its entry",
			repo: publishedRepo,
			change: func(t *testing.T, repo string) {
				require.NoError(t, os.Remove(filepath.Join(repo, "entries", "fleet", "111.0.2+up0.17.2.yaml")))
			},
			wantErr: "fleet 111.0.2+up0.17.2 is in index.yaml but has no entry: entries are never removed",
		},
		{
			name: "a published version lost its archive",
			repo: publishedRepo,
			change: func(t *testing.T, repo string) {
				require.NoError(t, os.Remove(filepath.Join(repo, "assets", "fleet", "fleet-111.0.2+up0.17.2.tgz")))
			},
			wantErr: "fleet 111.0.2+up0.17.2 is in index.yaml but has no archive in assets/",
		},
		{
			name: "an archive that has no entry",
			repo: publishedRepo,
			change: func(t *testing.T, repo string) {
				addArchive(t, repo, "fleet", "111.9.9+up9.9.9")
			},
			wantErr: "assets/ has an archive of fleet 111.9.9+up9.9.9 but there is no entry for it",
		},
		{
			name: "a new entry that already has an archive",
			repo: copyRepo,
			change: func(t *testing.T, repo string) {
				addArchive(t, repo, "fleet", "111.0.2+up0.17.2")
			},
			wantErr: "fleet 111.0.2+up0.17.2 is not in index.yaml yet but already has an archive in assets/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.repo(t)
			tt.change(t, repo)
			indexPath := filepath.Join(repo, "index.yaml")
			before, readErr := os.ReadFile(indexPath)

			res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

			require.NotZero(t, res.ExitCode)
			require.Contains(t, res.Stderr, tt.wantErr)
			after, afterErr := os.ReadFile(indexPath)
			require.Equal(t, readErr, afterErr, "the index.yaml is not created or removed")
			require.Equal(t, before, after, "the index.yaml is not changed")
		})
	}
}

func TestIndexHidesAPublishedVersion(t *testing.T) {
	repo := publishedRepo(t)
	before := readIndex(t, repo)
	hideEntry(t, repo, "fleet", "111.0.3+up0.17.4")

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.Zero(t, res.ExitCode, res.Stderr)
	after := readIndex(t, repo)
	require.NotContains(t, after, "version: 111.0.3+up0.17.4\n")
	require.Contains(t, after, "version: 111.0.3+up0.17.3\n")
	require.Equal(t, strings.Count(before, "    version: ")-1, strings.Count(after, "    version: "))
	require.NotContains(t, after, "hide", "hide is not a field of the index")
	require.FileExists(t, filepath.Join(repo, "entries", "fleet", "111.0.3+up0.17.4.yaml"))
	require.FileExists(t, filepath.Join(repo, "assets", "fleet", "fleet-111.0.3+up0.17.4.tgz"))

	res = harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, after, readIndex(t, repo), "a hidden version stays hidden")
}

func TestIndexRefusesToHideAVersionThatWasNeverPublished(t *testing.T) {
	repo := copyRepo(t)
	hideEntry(t, repo, "fleet", "111.0.3+up0.17.4")

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "fleet 111.0.3+up0.17.4 is hidden but was never published")
	require.NoFileExists(t, filepath.Join(repo, "index.yaml"))
}

func TestIndexDoesNotPublishAHiddenVersionAgain(t *testing.T) {
	repo := publishedRepo(t)
	hideEntry(t, repo, "fleet", "111.0.3+up0.17.4")
	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)
	require.Zero(t, res.ExitCode, res.Stderr)
	hidden := readIndex(t, repo)
	unhideEntry(t, repo, "fleet", "111.0.3+up0.17.4")

	res = harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "fleet 111.0.3+up0.17.4 is not in index.yaml yet but already has an archive in assets/")
	require.Equal(t, hidden, readIndex(t, repo))
}
