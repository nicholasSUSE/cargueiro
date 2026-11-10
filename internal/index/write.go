package index

import (
	"errors"
	"maps"
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
	var batch fsutil.Batch
	for _, ref := range slices.SortedFunc(maps.Keys(archives), compareRefs) {
		if err := batch.Write(filepath.Join(dir, archiveName(ref)), archives[ref]); err != nil {
			return nil, errors.Join(err, batch.Undo())
		}
	}
	return batch.Undo, nil
}
