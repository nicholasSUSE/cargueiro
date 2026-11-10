package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeFiles creates the given files, with the given content, under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
}

func TestReadPublished(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []Ref
	}{
		{
			name:  "no index.yaml",
			files: map[string]string{},
			want:  nil,
		},
		{
			name:  "an empty index.yaml",
			files: map[string]string{File: ""},
			want:  nil,
		},
		{
			name:  "an index.yaml of blank lines",
			files: map[string]string{File: "\n  \n"},
			want:  nil,
		},
		{
			name:  "an index without entries",
			files: map[string]string{File: "apiVersion: v1\nentries: {}\n"},
			want:  nil,
		},
		{
			name: "the versions of every chart",
			files: map[string]string{File: `apiVersion: v1
entries:
  fleet:
  - name: fleet
    version: 111.0.3+up0.17.3
  - name: fleet
    version: 111.0.2+up0.17.2
  fleet-crd:
  - name: fleet-crd
    version: 111.0.3+up0.17.3
`},
			want: []Ref{
				{Chart: "fleet", Version: "111.0.3+up0.17.3"},
				{Chart: "fleet", Version: "111.0.2+up0.17.2"},
				{Chart: "fleet-crd", Version: "111.0.3+up0.17.3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)

			got, err := readPublished(dir)

			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, indexRefs(got))
		})
	}
}

func TestReadPublishedFails(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "not YAML",
			content: "entries: [",
			wantErr: "read index.yaml",
		},
		{
			name: "a field that Helm's index does not know",
			content: `apiVersion: v1
entries:
  fleet:
  - name: fleet
    version: 1
    hide: true
`,
			wantErr: `unknown field "hide"`,
		},
		{
			name:    "no apiVersion",
			content: "entries: {}\n",
			wantErr: "read index.yaml: it has no apiVersion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{File: tt.content})

			got, err := readPublished(dir)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestArchiveRefs(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []Ref
	}{
		{
			name:  "no assets folder",
			files: map[string]string{},
			want:  nil,
		},
		{
			name:  "only logos",
			files: map[string]string{"assets/logos/fleet.svg": "<svg/>"},
			want:  nil,
		},
		{
			name: "the archive of every version, whatever the version holds",
			files: map[string]string{
				"assets/logos/fleet.svg":                          "<svg/>",
				"assets/fleet/fleet-111.0.3+up0.17.3.tgz":         "",
				"assets/fleet/fleet-111.0.4+up0.17.4-rc.1.tgz":    "",
				"assets/fleet-crd/fleet-crd-111.0.3+up0.17.3.tgz": "",
			},
			want: []Ref{
				{Chart: "fleet", Version: "111.0.3+up0.17.3"},
				{Chart: "fleet", Version: "111.0.4+up0.17.4-rc.1"},
				{Chart: "fleet-crd", Version: "111.0.3+up0.17.3"},
			},
		},
		{
			// Only an entry can tell this is not a version, and there is none.
			name:  "an archive of another chart is read as a version of this one",
			files: map[string]string{"assets/fleet/fleet-crd-111.0.3+up0.17.3.tgz": ""},
			want:  []Ref{{Chart: "fleet", Version: "crd-111.0.3+up0.17.3"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)

			got, err := archiveRefs(dir)

			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestArchiveRefsFails(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{
			name:    "a file in assets/ that is not in a chart folder",
			files:   map[string]string{"assets/README.md": ""},
			wantErr: "assets/README.md is not a chart directory",
		},
		{
			name:    "a file that is not an archive",
			files:   map[string]string{"assets/fleet/notes.txt": ""},
			wantErr: "assets/fleet/notes.txt is not an archive: it must be called fleet-<version>.tgz",
		},
		{
			name:    "an archive with no version",
			files:   map[string]string{"assets/fleet/fleet-.tgz": ""},
			wantErr: "assets/fleet/fleet-.tgz is not an archive",
		},
		{
			name:    "a provenance file",
			files:   map[string]string{"assets/fleet/fleet-111.0.3+up0.17.3.tgz.prov": ""},
			wantErr: "is not an archive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)

			got, err := archiveRefs(dir)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestArchiveRefsFailsOnAFolderInAChartFolder(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets", "fleet", "fleet-1.tgz"), 0o750))

	got, err := archiveRefs(dir)

	require.ErrorContains(t, err, "assets/fleet/fleet-1.tgz is not an archive")
	require.Nil(t, got)
}
