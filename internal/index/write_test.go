package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteArchives(t *testing.T) {
	dir := t.TempDir()
	archives := map[Ref][]byte{
		{Chart: "fleet", Version: "1"}:     []byte("one"),
		{Chart: "fleet", Version: "2"}:     []byte("two"),
		{Chart: "fleet-crd", Version: "1"}: []byte("crd"),
	}

	undo, err := writeArchives(dir, archives)

	require.NoError(t, err)
	for ref, want := range archives {
		got, err := os.ReadFile(filepath.Join(dir, archiveName(ref)))
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	require.NoError(t, undo())
	require.NoDirExists(t, filepath.Join(dir, "assets"), "the folders it created are removed too")
}

func TestWriteArchivesNothing(t *testing.T) {
	dir := t.TempDir()

	undo, err := writeArchives(dir, nil)

	require.NoError(t, err)
	require.NoError(t, undo())
	require.NoDirExists(t, filepath.Join(dir, "assets"))
}

func TestWriteArchivesUndoKeepsWhatWasThere(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"assets/logos/fleet.svg":   "<svg/>",
		"assets/fleet/fleet-0.tgz": "old",
		"assets/other/other-1.tgz": "other",
	})

	undo, err := writeArchives(dir, map[Ref][]byte{
		{Chart: "fleet", Version: "1"}:    []byte("new"),
		{Chart: "newchart", Version: "1"}: []byte("brand new"),
	})
	require.NoError(t, err)
	require.NoError(t, undo())

	require.FileExists(t, filepath.Join(dir, "assets", "logos", "fleet.svg"))
	require.FileExists(t, filepath.Join(dir, "assets", "fleet", "fleet-0.tgz"))
	require.FileExists(t, filepath.Join(dir, "assets", "other", "other-1.tgz"))
	require.NoFileExists(t, filepath.Join(dir, "assets", "fleet", "fleet-1.tgz"))
	require.NoDirExists(t, filepath.Join(dir, "assets", "newchart"), "a folder it created is removed")
}

func TestWriteArchivesLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()

	_, err := writeArchives(dir, map[Ref][]byte{{Chart: "fleet", Version: "1"}: []byte("one")})

	require.NoError(t, err)
	entries, err := os.ReadDir(filepath.Join(dir, "assets", "fleet"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "fleet-1.tgz", entries[0].Name())
}

func TestWriteArchivesFails(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{
			name:    "an archive that is there already",
			files:   map[string]string{"assets/fleet-crd/fleet-crd-1.tgz": "old"},
			wantErr: "assets/fleet-crd/fleet-crd-1.tgz already exists",
		},
		{
			name:    "a file where a folder is needed",
			files:   map[string]string{"assets/fleet-crd": "not a folder"},
			wantErr: "fleet-crd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)
			archives := map[Ref][]byte{
				{Chart: "fleet", Version: "1"}:     []byte("one"),
				{Chart: "fleet-crd", Version: "1"}: []byte("crd"),
			}

			undo, err := writeArchives(dir, archives)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, undo)
			require.NoDirExists(t, filepath.Join(dir, "assets", "fleet"), "what was written before the failure is removed")
			for name, content := range tt.files {
				got, readErr := os.ReadFile(filepath.Join(dir, name))
				require.NoError(t, readErr)
				require.Equal(t, content, string(got), "what was there is untouched")
			}
		})
	}
}
