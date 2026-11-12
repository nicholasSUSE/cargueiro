package index

import (
	"errors"
	"fmt"
	"time"

	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// checkCreated fails unless each of the added versions has the date its chart was
// pushed, naming every one that has none. Versions already published are not
// checked: they cannot change.
func checkCreated(all *repo.IndexFile, added []Ref) error {
	byRef := versionsByRef(all)

	var problems []error
	for _, ref := range added {
		if byRef[ref].Created.IsZero() {
			problems = append(problems, fmt.Errorf("%s %s has no created date: it must have the date its chart was pushed", ref.Chart, ref.Version))
		}
	}
	return errors.Join(problems...)
}

// newestCreated returns the latest created date among the versions in idx, or
// the zero time when none has one. It is the time the index was generated: it
// changes only when a version is added.
func newestCreated(idx *repo.IndexFile) time.Time {
	var newest time.Time
	for _, versions := range idx.Entries {
		for _, v := range versions {
			if v.Created.After(newest) {
				newest = v.Created
			}
		}
	}
	return newest
}
