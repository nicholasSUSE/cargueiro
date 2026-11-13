package audit

import (
	"fmt"

	"github.com/rancher/cargueiro/internal/index"
)

// checkURLs checks that the URL of every version in the index points to where
// the rules say the version is pulled from: the staging registry of the edition
// of its chart, with the tag the registry stores and the digest of the manifest
// pinned. index checks it only for a version it adds, so an entry and its line in
// the index that were edited together, to point somewhere else, would pass it.
func checkURLs(s *snapshot) (string, error) {
	description := fmt.Sprintf("The url of each of the %d versions in the index points to the staging registry of its chart.", len(s.versions))

	var found []error
	for _, v := range s.versions {
		rule, err := s.cfg.Find(v.chart)
		if err != nil {
			found = append(found, fmt.Errorf("%s: %w", versionName(v), err))
			continue
		}
		repository, err := s.cfg.StagingRepository(rule, s.registry)
		if err != nil {
			found = append(found, fmt.Errorf("%s: %w", versionName(v), err))
			continue
		}
		if err := index.CheckURL(repository, v.entry); err != nil {
			found = append(found, err)
		}
	}
	return description, problems(found)
}
