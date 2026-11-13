package index

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"
)

const (
	// AssetsDir is where the archives are, inside a charts repository.
	AssetsDir = "assets"
	// logosDir is the folder of AssetsDir that holds the logos, not charts.
	logosDir = "logos"
	// archiveSuffix ends the file name of every archive.
	archiveSuffix = ".tgz"
)

// ReadPublished returns the index.yaml of the charts repository in dir, as it
// is committed. A missing or empty file gives nil, because then nothing has been
// published yet. Fields it does not know are an error: they would be lost when
// the index is written again, and a published version never changes.
func ReadPublished(dir string) (*repo.IndexFile, error) {
	data, err := os.ReadFile(filepath.Join(dir, File))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", File, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var idx repo.IndexFile
	if err := yaml.UnmarshalStrict(data, &idx); err != nil {
		return nil, fmt.Errorf("read %s: %w", File, err)
	}
	if idx.APIVersion == "" {
		return nil, fmt.Errorf("read %s: it has no apiVersion", File)
	}
	return &idx, nil
}

// indexRefs returns the versions listed in idx, which can be nil.
func indexRefs(idx *repo.IndexFile) []Ref {
	if idx == nil {
		return nil
	}

	var refs []Ref
	for name, versions := range idx.Entries {
		for _, v := range versions {
			refs = append(refs, Ref{Chart: name, Version: v.Version})
		}
	}
	return refs
}

// archiveRefs returns the versions that have an archive,
// assets/<chart>/<chart>-<version>.tgz, in the charts repository in dir. The
// logos are not archives. Anything else in assets/ is an error, because it is
// not something the index accounts for.
func archiveRefs(dir string) ([]Ref, error) {
	assets := filepath.Join(dir, AssetsDir)
	charts, err := os.ReadDir(assets)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", AssetsDir, err)
	}

	var refs []Ref
	for _, c := range charts {
		if c.Name() == logosDir {
			continue
		}
		if !c.IsDir() {
			return nil, fmt.Errorf("%s/%s is not a chart directory", AssetsDir, c.Name())
		}

		files, err := os.ReadDir(filepath.Join(assets, c.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s/%s: %w", AssetsDir, c.Name(), err)
		}
		for _, f := range files {
			version, ok := archiveVersion(c.Name(), f.Name())
			if f.IsDir() || !ok {
				return nil, fmt.Errorf("%s/%s/%s is not an archive: it must be called %s-<version>%s", AssetsDir, c.Name(), f.Name(), c.Name(), archiveSuffix)
			}
			refs = append(refs, Ref{Chart: c.Name(), Version: version})
		}
	}
	return refs, nil
}

// archiveVersion returns the version in the file name of the archive of chart.
func archiveVersion(chart, file string) (string, bool) {
	rest, ok := strings.CutPrefix(file, chart+"-")
	if !ok {
		return "", false
	}
	version, ok := strings.CutSuffix(rest, archiveSuffix)
	return version, ok && version != ""
}

// readState collects what the entries, the published index and the archives in
// dir say about the versions of the charts repository. The published index is
// nil when nothing has been published yet.
func readState(dir string, e entries, published *repo.IndexFile) (State, error) {
	archives, err := archiveRefs(dir)
	if err != nil {
		return State{}, err
	}

	return State{
		Entries:   indexRefs(e.all),
		Hidden:    e.hidden,
		Published: indexRefs(published),
		Archives:  archives,
	}, nil
}
