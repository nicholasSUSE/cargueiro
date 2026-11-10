package index

import (
	"fmt"
	"regexp"
	"strings"

	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// digestForm is a pinned digest: the algorithm and the hex of the sha256.
var digestForm = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// checkURL fails unless the only URL of the entry is where its version is
// pulled from: oci://<repository>:<version>@sha256:<digest>, with the version
// written as the entry has it and the digest of the manifest pinned.
func checkURL(repository string, entry *repo.ChartVersion) error {
	if len(entry.URLs) != 1 {
		return fmt.Errorf("%s %s has %d urls: it must have exactly one", entry.Name, entry.Version, len(entry.URLs))
	}

	prefix := "oci://" + repository + ":" + entry.Version + "@"
	digest, found := strings.CutPrefix(entry.URLs[0], prefix)
	if !found || !digestForm.MatchString(digest) {
		return fmt.Errorf("%s %s has the url %q, but it must be %ssha256:<digest>", entry.Name, entry.Version, entry.URLs[0], prefix)
	}
	return nil
}
