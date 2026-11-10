// Package fsutil holds the few file operations that cargueiro commands share
// when they write into a repository: folders made one at a time, so that what
// was made can be taken back, and files that are never replaced.
package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// MakeFolders creates dir and the folders above it that are missing, and returns
// the ones it created, the highest first.
func MakeFolders(dir string) ([]string, error) {
	var missing []string
	for path := dir; ; path = filepath.Dir(path) {
		_, err := os.Stat(path)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		missing = append(missing, path)
	}

	var created []string
	for _, path := range slices.Backward(missing) {
		if err := os.Mkdir(path, 0o750); err != nil {
			return created, err
		}
		created = append(created, path)
	}
	return created, nil
}

// WriteNewFile writes data to path, through a temporary file so that the file is
// there whole or not at all. It fails if path exists.
func WriteNewFile(path string, data []byte) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".new-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return errors.Join(err, IgnoreMissing(os.Remove(tmp.Name())))
	}
	return nil
}

// IgnoreMissing returns err, unless it says that something was not there.
func IgnoreMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
