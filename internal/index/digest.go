package index

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// archiveName returns where the archive of a version is, inside the charts
// repository: assets/<chart>/<chart>-<version>.tgz.
func archiveName(ref Ref) string {
	return path.Join(AssetsDir, ref.Chart, ref.Chart+"-"+ref.Version+archiveSuffix)
}

// archiveDigests returns the sha256, in hex, of the archive of each version in
// all that has one in the charts repository in dir.
func archiveDigests(dir string, all *repo.IndexFile) (map[Ref]string, error) {
	digests := make(map[Ref]string)
	for ref := range versionsByRef(all) {
		sum, err := fileDigest(filepath.Join(dir, archiveName(ref)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("digest of %s %s: %w", ref.Chart, ref.Version, err)
		}
		digests[ref] = sum
	}
	return digests, nil
}

// fileDigest returns the sha256 of the file at path, in hex.
func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// setDigests puts the digest of each version in digests on its entry in all.
func setDigests(all *repo.IndexFile, digests map[Ref]string) {
	for name, versions := range all.Entries {
		for _, v := range versions {
			if sum, found := digests[Ref{Chart: name, Version: v.Version}]; found {
				v.Digest = sum
			}
		}
	}
}

// checkArchivesUnchanged fails, naming every version at fault, when the archive
// of a published version is not the one its digest in published says. A version
// without an archive in digests is left to State.Plan.
func checkArchivesUnchanged(published *repo.IndexFile, digests map[Ref]string) error {
	if published == nil {
		return nil
	}

	refs := indexRefs(published)
	slices.SortFunc(refs, compareRefs)
	byRef := versionsByRef(published)

	var problems []error
	for _, ref := range refs {
		now, found := digests[ref]
		if !found || byRef[ref].Digest == now {
			continue
		}
		problems = append(problems, fmt.Errorf("%s %s: %s has changed since it was published: index.yaml has the digest %s and the file has %s. A published archive is never allowed to change",
			ref.Chart, ref.Version, archiveName(ref), byRef[ref].Digest, now))
	}
	return errors.Join(problems...)
}
