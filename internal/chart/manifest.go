package chart

import (
	"encoding/json"
	"fmt"
	"strings"

	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"

	"github.com/nicholasSUSE/cargueiro/internal/integrity"
)

// DirManifest returns the digest of every file that Helm puts in the archive of
// the chart in dir. It reads the chart the way Helm does, so the files that
// .helmignore lists are not there.
//
// Helm writes Chart.yaml again when it packages, so that file is not taken as
// it is on disk: its digest is the one of the metadata Helm understands, which
// is what both sides of a comparison have. A dependency is a chart of its own,
// whether it is a directory or a packaged .tgz in charts/, because Helm writes
// every one as a directory in the archive: its files are named
// charts/<name>/<file>.
func DirManifest(dir string) (*integrity.Manifest, error) {
	c, err := loader.LoadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read the chart in %s: %w", dir, err)
	}
	return manifestOf(c)
}

// ArchiveManifest returns the digest of every file in the chart archive at
// path, named as DirManifest names them.
func ArchiveManifest(path string) (*integrity.Manifest, error) {
	c, err := loader.LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the chart archive %s: %w", path, err)
	}
	return manifestOf(c)
}

// manifestOf returns the manifest of a chart and its dependencies.
func manifestOf(c *chartv2.Chart) (*integrity.Manifest, error) {
	m := integrity.NewManifest()
	if err := addChart(m, c, ""); err != nil {
		return nil, err
	}
	return m, nil
}

// addChart adds the files of c, and of its dependencies, to m. prefix is the
// path of c inside the archive.
func addChart(m *integrity.Manifest, c *chartv2.Chart, prefix string) error {
	metadata, err := json.Marshal(c.Metadata)
	if err != nil {
		return fmt.Errorf("read the metadata of %s: %w", c.Name(), err)
	}
	if err := m.Add(prefix+"Chart.yaml", integrity.Bytes(metadata)); err != nil {
		return err
	}

	for _, f := range c.Raw {
		// Chart.yaml is the metadata above, and what is under charts/ belongs to
		// the dependencies, which are read as charts below.
		if f.Name == "Chart.yaml" || strings.HasPrefix(f.Name, "charts/") {
			continue
		}
		if err := m.Add(prefix+f.Name, integrity.Bytes(f.Data)); err != nil {
			return err
		}
	}

	for _, dep := range c.Dependencies() {
		if err := addChart(m, dep, prefix+"charts/"+dep.Name()+"/"); err != nil {
			return err
		}
	}
	return nil
}
