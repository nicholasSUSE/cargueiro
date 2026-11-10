package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeFolders(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c")

	created, err := MakeFolders(deep)

	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(dir, "a"), filepath.Join(dir, "a", "b"), deep}, created, "the highest first")
	require.DirExists(t, deep)
}

func TestMakeFoldersKeepsWhatIsThere(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0o750))

	created, err := MakeFolders(filepath.Join(dir, "a"))

	require.NoError(t, err)
	require.Empty(t, created)
}

func TestMakeFoldersFails(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), nil, 0o600))

	created, err := MakeFolders(filepath.Join(dir, "file", "inside"))

	require.Error(t, err)
	require.Empty(t, created)
}

func TestWriteNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	require.NoError(t, WriteNewFile(path, []byte("content")))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "content", string(got))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temporary file is left")
}

func TestWriteNewFileNeverReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.txt")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))

	err := WriteNewFile(path, []byte("new"))

	require.ErrorContains(t, err, "old.txt already exists")
	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, "old", string(got))
}

func TestWriteNewFileFails(t *testing.T) {
	err := WriteNewFile(filepath.Join(t.TempDir(), "missing", "new.txt"), []byte("x"))

	require.Error(t, err)
}

func TestIgnoreMissing(t *testing.T) {
	other := errors.New("other")

	require.NoError(t, IgnoreMissing(nil))
	require.NoError(t, IgnoreMissing(fs.ErrNotExist))
	require.NoError(t, IgnoreMissing(&fs.PathError{Op: "remove", Path: "x", Err: fs.ErrNotExist}))
	require.Equal(t, other, IgnoreMissing(other))
}
