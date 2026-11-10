package index

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
)

// writeArchives writes each archive to assets/<chart>/<chart>-<version>.tgz in
// the charts repository in dir, creating the folders it needs. It never
// replaces an archive that is there.
//
// It returns a function that takes away everything it wrote, folders it created
// included. When it fails it has taken away what it wrote already, and returns
// no function.
func writeArchives(dir string, archives map[Ref][]byte) (undo func() error, err error) {
	var files, folders []string
	undo = func() error {
		var problems []error
		for _, file := range slices.Backward(files) {
			problems = append(problems, ignoreMissing(os.Remove(file)))
		}
		for _, folder := range slices.Backward(folders) {
			problems = append(problems, ignoreMissing(os.Remove(folder)))
		}
		return errors.Join(problems...)
	}

	for _, ref := range slices.SortedFunc(maps.Keys(archives), compareRefs) {
		path := filepath.Join(dir, archiveName(ref))

		created, err := makeFolders(filepath.Dir(path))
		folders = append(folders, created...)
		if err == nil {
			err = writeNewFile(path, archives[ref])
		}
		if err != nil {
			return nil, errors.Join(err, undo())
		}
		files = append(files, path)
	}
	return undo, nil
}

// makeFolders creates dir and the folders above it that are missing, and returns
// the ones it created, the highest first.
func makeFolders(dir string) ([]string, error) {
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

// writeNewFile writes data to path, through a temporary file so that the file is
// there whole or not at all. It fails if path exists.
func writeNewFile(path string, data []byte) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".archive-*")
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
		return errors.Join(err, ignoreMissing(os.Remove(tmp.Name())))
	}
	return nil
}

// ignoreMissing returns err, unless it says that something was not there.
func ignoreMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
