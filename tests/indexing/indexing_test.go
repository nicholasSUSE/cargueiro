//go:build e2e

// Package indexing holds the e2e tests of `cargueiro index`, run against the
// real binary and the charts repository kept in tests/data/rancher-charts. Each
// test works on a copy of it, in a temporary directory.
package indexing

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
// archives are empty files, and the index has their digest.
func publishedRepo(t *testing.T) string {
	t.Helper()

	repo := copyRepo(t)
	want, err := os.ReadFile(filepath.Join(fixture(t), "index.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "index.yaml"), []byte(withDigests(string(want))), 0o600))

	addArchives(t, repo)
	return repo
}

// emptyDigest is the sha256 of nothing, which is what the empty archives of
// these tests have.
const emptyDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// withDigests adds the digest of an empty archive to every line of an index.
func withDigests(index string) string {
	return regexp.MustCompile(`(?m)^(    description: .*\n)`).ReplaceAllString(index, "${1}    digest: "+emptyDigest+"\n")
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
	require.Equal(t, "hide fleet 111.0.3+up0.17.4\n", res.Stdout)
	after := readIndex(t, repo)
	require.NotContains(t, after, "version: 111.0.3+up0.17.4\n")
	require.Contains(t, after, "version: 111.0.3+up0.17.3\n")
	require.Equal(t, strings.Count(before, "    version: ")-1, strings.Count(after, "    version: "))
	require.NotContains(t, after, "hide", "hide is not a field of the index")
	require.FileExists(t, filepath.Join(repo, "entries", "fleet", "111.0.3+up0.17.4.yaml"))
	require.FileExists(t, filepath.Join(repo, "assets", "fleet", "fleet-111.0.3+up0.17.4.tgz"))

	res = harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "nothing to change\n", res.Stdout, "a hidden version is hidden once")
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

func TestIndexRefusesToChangeAPublishedVersion(t *testing.T) {
	const version = "111.0.3+up0.17.3"

	tests := []struct {
		name string
		old  string
		new  string
		hide bool
	}{
		{
			name: "a description that changed",
			old:  "description: Fixture of fleet\n",
			new:  "description: Another description\n",
		},
		{
			name: "a URL that changed",
			old:  "oci://stgregistry.suse.com/rancher/catalog/fleet:",
			new:  "oci://stgregistry.suse.com/rancher/catalog/fleet-other:",
		},
		{
			name: "an icon that changed",
			old:  "icon: file://assets/logos/fleet.svg\n",
			new:  "icon: file://assets/logos/other.svg\n",
		},
		{
			name: "a description that changed together with hide",
			old:  "description: Fixture of fleet\n",
			new:  "description: Another description\n",
			hide: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := publishedRepo(t)
			path := filepath.Join(repo, "entries", "fleet", version+".yaml")
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Contains(t, string(data), tt.old)
			require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), tt.old, tt.new, 1)), 0o600))
			if tt.hide {
				hideEntry(t, repo, "fleet", version)
			}
			before := readIndex(t, repo)

			res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

			require.NotZero(t, res.ExitCode)
			require.Contains(t, res.Stderr, "fleet "+version+" is in index.yaml and its entry no longer matches it: a published version never changes")
			require.Equal(t, before, readIndex(t, repo))
		})
	}
}

// pinnedDigest matches the digest a URL of the fixture is pinned to.
var pinnedDigest = regexp.MustCompile(`@sha256:[0-9a-f]{64}`)

// editFile changes a file with fn.
func editFile(t *testing.T, path string, fn func(string) string) {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(fn(string(data))), 0o600))
}

func TestIndexRefusesTheUrlOfANewEntry(t *testing.T) {
	const version = "111.0.2+up0.17.2"

	tests := []struct {
		name    string
		edit    func(string) string
		wantErr string
	}{
		{
			name:    "a url with no digest",
			edit:    func(s string) string { return pinnedDigest.ReplaceAllString(s, "") },
			wantErr: "fleet " + version + " has the url",
		},
		{
			name:    "a url of another registry",
			edit:    func(s string) string { return strings.Replace(s, "stgregistry.suse.com", "registry.suse.com", 1) },
			wantErr: "but it must be oci://stgregistry.suse.com/rancher/catalog/fleet:" + version + "@sha256:<digest>",
		},
		{
			name:    "a url that has the tag form of the version",
			edit:    func(s string) string { return strings.Replace(s, ":111.0.2+up0.17.2@", ":111.0.2_up0.17.2@", 1) },
			wantErr: "but it must be oci://stgregistry.suse.com/rancher/catalog/fleet:" + version + "@sha256:<digest>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := copyRepo(t)
			editFile(t, filepath.Join(repo, "entries", "fleet", version+".yaml"), tt.edit)

			res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

			require.NotZero(t, res.ExitCode)
			require.Contains(t, res.Stderr, tt.wantErr)
			require.NoFileExists(t, filepath.Join(repo, "index.yaml"))
		})
	}
}

func TestIndexDoesNotCheckTheUrlOfAPublishedVersion(t *testing.T) {
	repo := publishedRepo(t)
	fleetURL := regexp.MustCompile(`(fleet:111\.0\.2\+up0\.17\.2)@sha256:[0-9a-f]{64}`)
	dropDigest := func(s string) string { return fleetURL.ReplaceAllString(s, "$1") }
	editFile(t, filepath.Join(repo, "entries", "fleet", "111.0.2+up0.17.2.yaml"), dropDigest)
	editFile(t, filepath.Join(repo, "index.yaml"), dropDigest)
	before := readIndex(t, repo)

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, before, readIndex(t, repo))
}

func TestIndexRefusesTheIconOfANewEntry(t *testing.T) {
	tests := []struct {
		name    string
		edit    func(t *testing.T, repo string)
		wantErr string
	}{
		{
			name: "a logo that does not exist",
			edit: func(t *testing.T, repo string) {
				require.NoError(t, os.Remove(filepath.Join(repo, "assets", "logos", "longhorn.svg")))
			},
			wantErr: "longhorn 111.0.1+up1.10.1 has the icon \"file://assets/logos/longhorn.svg\", but assets/logos/longhorn.svg does not exist",
		},
		{
			name: "an icon on the web",
			edit: func(t *testing.T, repo string) {
				editFile(t, filepath.Join(repo, "entries", "longhorn", "111.0.1+up1.10.1.yaml"), func(s string) string {
					return strings.Replace(s, "file://assets/logos/longhorn.svg", "https://charts.rancher.io/assets/logos/longhorn.svg", 1)
				})
			},
			wantErr: "but it must be file://assets/logos/<file>",
		},
		{
			name: "no icon",
			edit: func(t *testing.T, repo string) {
				editFile(t, filepath.Join(repo, "entries", "longhorn", "111.0.1+up1.10.1.yaml"), func(s string) string {
					return strings.Replace(s, "icon: file://assets/logos/longhorn.svg\n", "", 1)
				})
			},
			wantErr: "longhorn 111.0.1+up1.10.1 has the icon \"\", but it must be file://assets/logos/<file>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := copyRepo(t)
			tt.edit(t, repo)

			res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

			require.NotZero(t, res.ExitCode)
			require.Contains(t, res.Stderr, tt.wantErr)
			require.NoFileExists(t, filepath.Join(repo, "index.yaml"))
		})
	}
}

func TestIndexDoesNotCheckTheIconOfAPublishedVersion(t *testing.T) {
	repo := publishedRepo(t)
	require.NoError(t, os.Remove(filepath.Join(repo, "assets", "logos", "longhorn.svg")))
	before := readIndex(t, repo)

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, before, readIndex(t, repo))
}

func TestIndexRefusesToChangeAPublishedArchive(t *testing.T) {
	repo := publishedRepo(t)
	archive := filepath.Join(repo, "assets", "fleet", "fleet-111.0.3+up0.17.3.tgz")
	require.NoError(t, os.WriteFile(archive, []byte("not what was published"), 0o600))
	before := readIndex(t, repo)

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo)

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "fleet 111.0.3+up0.17.3: assets/fleet/fleet-111.0.3+up0.17.3.tgz has changed since it was published")
	require.Contains(t, res.Stderr, "index.yaml has the digest "+emptyDigest+" and the file has ")
	require.Contains(t, res.Stderr, "A published archive is never allowed to change")
	require.Equal(t, before, readIndex(t, repo))
}
