package audit

import "fmt"

// checkArchives checks that the digest of every version in the index is the
// digest of the chart layer in its manifest in the registry, which is the sha256
// of the archive. Helm checks it when it pulls a chart, and so does a Rancher
// that pulls it from the index. A version the registry could not give is left
// to checkManifests.
func checkArchives(s *snapshot) (string, error) {
	description := fmt.Sprintf("The digest of each of the %d versions in the index is the digest of the chart in its manifest in the registry.", len(s.versions))

	var found []error
	for _, v := range s.versions {
		got := s.remote[versionName(v)]
		if got.err != nil {
			continue
		}
		if want := "sha256:" + v.entry.Digest; got.info.ChartDigest != want {
			found = append(found, fmt.Errorf("%s: the index has the digest %s, but the chart in the registry has %s", versionName(v), want, got.info.ChartDigest))
		}
	}
	return description, problems(found)
}
