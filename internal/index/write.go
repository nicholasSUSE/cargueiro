package index

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/nicholasSUSE/cargueiro/internal/fsutil"
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
			problems = append(problems, fsutil.IgnoreMissing(os.Remove(file)))
		}
		for _, folder := range slices.Backward(folders) {
			problems = append(problems, fsutil.IgnoreMissing(os.Remove(folder)))
		}
		return errors.Join(problems...)
	}

	for _, ref := range slices.SortedFunc(maps.Keys(archives), compareRefs) {
		path := filepath.Join(dir, archiveName(ref))

		created, err := fsutil.MakeFolders(filepath.Dir(path))
		folders = append(folders, created...)
		if err == nil {
			err = fsutil.WriteNewFile(path, archives[ref])
		}
		if err != nil {
			return nil, errors.Join(err, undo())
		}
		files = append(files, path)
	}
	return undo, nil
}
