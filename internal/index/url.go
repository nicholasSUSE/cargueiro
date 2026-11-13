package index

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	repo "helm.sh/helm/v4/pkg/repo/v1"

	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/version"
)

// digestForm is a pinned digest: the algorithm and the hex of the sha256.
var digestForm = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// CheckURL fails unless the only URL of the entry is where its version is
// pulled from: oci://<repository>:<tag>@sha256:<digest>, with the tag written as
// the registry stores the version, a "+" as a "_", and the digest of the manifest
// pinned. The version of the entry keeps its "+".
func CheckURL(repository string, entry *repo.ChartVersion) error {
	if len(entry.URLs) != 1 {
		return fmt.Errorf("%s %s has %d urls: it must have exactly one", entry.Name, entry.Version, len(entry.URLs))
	}

	prefix := "oci://" + repository + ":" + version.ToOCITag(entry.Version) + "@"
	digest, found := strings.CutPrefix(entry.URLs[0], prefix)
	if !found || !digestForm.MatchString(digest) {
		return fmt.Errorf("%s %s has the url %q, but it must be %ssha256:<digest>", entry.Name, entry.Version, entry.URLs[0], prefix)
	}
	return nil
}

// checkURLs checks the URL of each of the added versions, naming every one that
// is wrong. registry replaces the staging registry of the chart's edition when
// it is not empty, as host[:port]/path. Versions already published are not
// checked: they cannot change.
func checkURLs(cfg *rules.Config, registry string, all *repo.IndexFile, added []Ref) error {
	byRef := versionsByRef(all)

	var problems []error
	for _, ref := range added {
		rule, err := cfg.Find(ref.Chart)
		if err != nil {
			return err
		}
		repository, err := cfg.StagingRepository(rule, registry)
		if err != nil {
			return err
		}
		if err := CheckURL(repository, byRef[ref]); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}
