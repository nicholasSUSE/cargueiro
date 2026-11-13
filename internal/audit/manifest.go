package audit

import "fmt"

// checkManifests checks that the tag of every version in the registry names the
// manifest that the url of the version pins. A tag can be pushed again with
// other content, and a url can be edited, and then they disagree. A tag the
// registry does not have fails here too, so a version that was never pushed is
// found by this check.
func checkManifests(s *snapshot) (string, error) {
	description := fmt.Sprintf("The tag of each of the %d versions in the index names, in the registry, the manifest that its url pins.", len(s.versions))

	var found []error
	for _, v := range s.versions {
		got := s.remote[versionName(v)]
		if got.err != nil {
			found = append(found, fmt.Errorf("%s: %w", versionName(v), got.err))
			continue
		}
		pinned := pinnedDigest(v)
		if pinned == "" {
			found = append(found, fmt.Errorf("%s: its url pins no digest to compare with", versionName(v)))
			continue
		}
		if got.info.ManifestDigest != pinned {
			found = append(found, fmt.Errorf("%s: the tag %s of %s/%s is the manifest %s in the registry, but the url pins %s",
				versionName(v), got.ref.Tag, got.ref.Host, got.ref.Repository, got.info.ManifestDigest, pinned))
		}
	}
	return description, problems(found)
}
