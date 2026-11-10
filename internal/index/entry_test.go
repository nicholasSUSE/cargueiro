package index

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const entryYAML = `apiVersion: v2
name: fleet
urls:
- oci://registry.example.com/fleet:111.0.3
version: 111.0.3
`

func TestReadEntry(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantHide bool
	}{
		{
			name:     "an entry without hide",
			content:  entryYAML,
			wantHide: false,
		},
		{
			name:     "hide: true",
			content:  entryYAML + "hide: true\n",
			wantHide: true,
		},
		{
			name:     "hide: false",
			content:  entryYAML + "hide: false\n",
			wantHide: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"111.0.3.yaml": tt.content})

			got, err := readEntry(filepath.Join(dir, "111.0.3.yaml"), "fleet", "111.0.3.yaml")

			require.NoError(t, err)
			require.Equal(t, "fleet", got.Name)
			require.Equal(t, "111.0.3", got.Version)
			require.Equal(t, []string{"oci://registry.example.com/fleet:111.0.3"}, got.URLs)
			require.Equal(t, tt.wantHide, got.Hide)
		})
	}
}

func TestReadEntryFails(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		wantErr string
	}{
		{
			name:    "a field the index does not know",
			file:    "111.0.3.yaml",
			content: entryYAML + "hidden: true\n",
			wantErr: `unknown field "hidden"`,
		},
		{
			name:    "hide that is not a boolean",
			file:    "111.0.3.yaml",
			content: entryYAML + "hide: soon\n",
			wantErr: "entry fleet/111.0.3.yaml",
		},
		{
			name:    "an empty entry",
			file:    "111.0.3.yaml",
			content: "{}\n",
			wantErr: "entry fleet/111.0.3.yaml is empty",
		},
		{
			name:    "the entry of another chart",
			file:    "111.0.3.yaml",
			content: "apiVersion: v2\nname: fleet-crd\nversion: 111.0.3\n",
			wantErr: `entry fleet/111.0.3.yaml is of chart "fleet-crd"`,
		},
		{
			name:    "a file named for another version",
			file:    "111.0.9.yaml",
			content: entryYAML,
			wantErr: "entry fleet/111.0.9.yaml holds version 111.0.3, so the file must be called 111.0.3.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{tt.file: tt.content})

			got, err := readEntry(filepath.Join(dir, tt.file), "fleet", tt.file)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}
