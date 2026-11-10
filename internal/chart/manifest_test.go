package chart

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/integrity"
)

// writeTree writes files, named by their path inside dir.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, body := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
}

// packageDir packages dir into a new directory and returns the archive.
func packageDir(t *testing.T, dir string) string {
	t.Helper()

	archive, err := Package(context.Background(), dir, t.TempDir())
	require.NoError(t, err)
	return archive
}

// requireSame checks that the files of a chart directory and of the archive
// that Helm makes from it have the same digests.
func requireSame(t *testing.T, dir string) (dirManifest *integrity.Manifest) {
	t.Helper()

	archive := packageDir(t, dir)
	fromDir, err := DirManifest(dir)
	require.NoError(t, err)
	fromArchive, err := ArchiveManifest(archive)
	require.NoError(t, err)

	require.Empty(t, integrity.Compare(fromDir, fromArchive), "the directory and its archive have the same files")
	return fromDir
}

func TestTheManifestsOfADirectoryAndItsArchiveAreTheSame(t *testing.T) {
	const chartYAML = "apiVersion: v2\nname: top\nversion: 1.0.0\n"

	tests := []struct {
		name      string
		files     map[string]string
		wantNames []string
	}{
		{
			name: "a chart",
			files: map[string]string{
				"Chart.yaml":          chartYAML,
				"values.yaml":         "a: 1\n",
				"templates/a.yaml":    "kind: A\n",
				"templates/_help.tpl": "{{/* x */}}\n",
			},
			wantNames: []string{"Chart.yaml", "templates/_help.tpl", "templates/a.yaml", "values.yaml"},
		},
		{
			name: "a Chart.yaml with comments, order and quotes that Helm does not keep",
			files: map[string]string{
				"Chart.yaml": "# a comment\nversion:   \"1.0.0\"\nname: 'top'\napiVersion: v2\nannotations:\n  b: \"2\"\n  a: 1\n",
			},
			wantNames: []string{"Chart.yaml"},
		},
		{
			name: "a file that .helmignore lists",
			files: map[string]string{
				"Chart.yaml":       chartYAML,
				".helmignore":      "*.bak\n",
				"templates/a.yaml": "kind: A\n",
				"templates/a.bak":  "old\n",
			},
			wantNames: []string{".helmignore", "Chart.yaml", "templates/a.yaml"},
		},
		{
			name: "a dependency that is a directory",
			files: map[string]string{
				"Chart.yaml":                  chartYAML + "dependencies:\n  - name: sub\n    version: 0.1.0\n",
				"templates/a.yaml":            "kind: A\n",
				"charts/sub/Chart.yaml":       "# comment\napiVersion: v2\nname: sub\nversion: 0.1.0\n",
				"charts/sub/templates/s.yaml": "kind: S\n",
			},
			wantNames: []string{"Chart.yaml", "charts/sub/Chart.yaml", "charts/sub/templates/s.yaml", "templates/a.yaml"},
		},
		{
			name: "files of other kinds",
			files: map[string]string{
				"Chart.yaml":         chartYAML,
				"README.md":          "readme\n",
				"values.schema.json": "{}\n",
				"files/data.txt":     "data\n",
			},
			wantNames: []string{"Chart.yaml", "README.md", "files/data.txt", "values.schema.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, tt.files)

			got := requireSame(t, dir)

			require.Equal(t, tt.wantNames, got.Names())
		})
	}
}

func TestADependencyThatIsAPackagedArchiveIsReadAsTheDirectoryHelmWrites(t *testing.T) {
	sub := t.TempDir()
	writeTree(t, sub, map[string]string{
		"Chart.yaml":       "apiVersion: v2\nname: sub\nversion: 0.1.0\n",
		"templates/s.yaml": "kind: S\n",
	})
	subArchive := packageDir(t, sub)

	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"Chart.yaml":       "apiVersion: v2\nname: top\nversion: 1.0.0\ndependencies:\n  - name: sub\n    version: 0.1.0\n",
		"templates/a.yaml": "kind: A\n",
	})
	data, err := os.ReadFile(subArchive)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "charts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "charts", "sub-0.1.0.tgz"), data, 0o600))

	got := requireSame(t, dir)

	require.Equal(t, []string{"Chart.yaml", "charts/sub/Chart.yaml", "charts/sub/templates/s.yaml", "templates/a.yaml"}, got.Names())
}

func TestTheRealFleetChartsHaveTheSameFilesInTheirArchive(t *testing.T) {
	for _, name := range []string{"fleet", "fleet-agent", "fleet-crd"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("..", "..", "tests", "data", "charts", "fleet", "src", name)

			got := requireSame(t, dir)

			require.NotZero(t, got.Len())
		})
	}
}

func TestADifferenceBetweenTheDirectoryAndTheArchiveIsFound(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"Chart.yaml":       "apiVersion: v2\nname: top\nversion: 1.0.0\ndescription: one\n",
		"values.yaml":      "a: 1\n",
		"templates/a.yaml": "kind: A\n",
		"templates/b.yaml": "kind: B\n",
	})
	archive := packageDir(t, dir)

	// The chart changes after it was packaged.
	writeTree(t, dir, map[string]string{
		"templates/a.yaml": "kind: A\nchanged: true\n",
		"Chart.yaml":       "apiVersion: v2\nname: top\nversion: 1.0.0\ndescription: two\n",
		"templates/c.yaml": "kind: C\n",
	})
	require.NoError(t, os.Remove(filepath.Join(dir, "templates", "b.yaml")))

	fromDir, err := DirManifest(dir)
	require.NoError(t, err)
	fromArchive, err := ArchiveManifest(archive)
	require.NoError(t, err)

	diffs := integrity.Compare(fromDir, fromArchive)
	var got []string
	for _, d := range diffs {
		got = append(got, d.Name+": "+string(d.Kind))
	}
	require.Equal(t, []string{
		"Chart.yaml: different",
		"templates/a.yaml: different",
		"templates/b.yaml: only in the second",
		"templates/c.yaml: only in the first",
	}, got)
}

func TestManifestsFail(t *testing.T) {
	t.Run("a directory without Chart.yaml", func(t *testing.T) {
		m, err := DirManifest(t.TempDir())

		require.ErrorContains(t, err, "read the chart in")
		require.Nil(t, m)
	})

	t.Run("a file that is not an archive", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "x.tgz")
		require.NoError(t, os.WriteFile(path, []byte("this is not an archive"), 0o600))

		m, err := ArchiveManifest(path)

		require.ErrorContains(t, err, "read the chart archive")
		require.Nil(t, m)
	})

	t.Run("an archive that does not exist", func(t *testing.T) {
		m, err := ArchiveManifest(filepath.Join(t.TempDir(), "nope.tgz"))

		require.Error(t, err)
		require.Nil(t, m)
	})
}
