// Package stamp puts the marks Rancher needs on a chart directory: the final
// version and the helm.sh/images annotation. It validates before it changes
// anything, and changes only those two places of Chart.yaml.
package stamp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/nicholasSUSE/cargueiro/internal/annotations"
	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
	"github.com/nicholasSUSE/cargueiro/internal/version"
)

// helmImages is the annotation that lists the images of a chart.
const helmImages = "helm.sh/images"

// Plan is what stamping a chart directory would change. Building it writes
// nothing.
type Plan struct {
	// Path is the Chart.yaml that Write replaces.
	Path string
	// Before and After are Chart.yaml as it is and as it would be.
	Before, After []byte
	// OldVersion and NewVersion are the version of the chart before and after.
	OldVersion, NewVersion string
	// AddedImages are the names of the entries added to helm.sh/images.
	AddedImages []string
}

// Changed reports whether stamping changes Chart.yaml.
func (p *Plan) Changed() bool {
	return !bytes.Equal(p.Before, p.After)
}

// Build works out what stamping dir with newVersion would change. The chart
// must be listed in cfg. It fails, and says why, before anything is written,
// when the version does not follow the rules, when the chart is for a Rancher
// version the new system does not support, or when an image of values.yaml
// cannot be listed in helm.sh/images.
func Build(cfg *rules.Config, dir, newVersion string) (*Plan, error) {
	files, err := chart.Load(dir)
	if err != nil {
		return nil, err
	}
	rule, err := cfg.Find(files.Metadata.Name)
	if err != nil {
		return nil, err
	}
	if err := checkVersion(cfg, rule, files, newVersion); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "Chart.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	after, err := chart.SetVersion(before, newVersion)
	if err != nil {
		return nil, err
	}

	value, changed, problems := annotations.GenerateHelmImages(files.Annotations, files.Values)
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "\n"))
	}
	var added []string
	if changed {
		if after, err = chart.SetAnnotation(after, helmImages, value); err != nil {
			return nil, err
		}
		if added, err = newEntries(files.Annotations[helmImages], value); err != nil {
			return nil, err
		}
	}

	return &Plan{
		Path:        path,
		Before:      before,
		After:       after,
		OldVersion:  files.Metadata.Version,
		NewVersion:  newVersion,
		AddedImages: added,
	}, nil
}

// Write replaces Chart.yaml with the planned one, through a temporary file in
// the same directory, so that it is never left half written. It does nothing
// when nothing changes.
func (p *Plan) Write() error {
	if !p.Changed() {
		return nil
	}

	info, err := os.Stat(p.Path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", p.Path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.Path), ".Chart.yaml.*")
	if err != nil {
		return fmt.Errorf("write %s: %w", p.Path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone after the rename; this is for a failure

	if _, err := tmp.Write(p.After); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", p.Path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", p.Path, err)
	}
	if err := os.Chmod(tmp.Name(), info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", p.Path, err)
	}
	if err := os.Rename(tmp.Name(), p.Path); err != nil {
		return fmt.Errorf("write %s: %w", p.Path, err)
	}
	return nil
}

// checkVersion validates newVersion for the chart. A CRD chart has no
// rancher-version, so its version is checked for its form only. Any other chart
// needs a rancher-version with a lower bound, which must be a Rancher version
// the new system supports, and the major of the version must be the one the
// rules tie to it.
func checkVersion(cfg *rules.Config, rule rules.Chart, files *chart.Files, newVersion string) error {
	versions := cfg.Versions()
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return err
	}
	if err := policy.Validate(newVersion, rule.PlainVersion); err != nil {
		return err
	}
	if rule.Kind == rules.KindCRD {
		return nil
	}

	rancherVersion := annotations.RancherVersionAnnotation(files.Annotations)
	rancher, ok := version.RancherOf(rancherVersion)
	if !ok {
		return fmt.Errorf("catalog.cattle.io/rancher-version %q has no lower bound like \">= 2.%d.0-0\": stamp needs it to know the major of the version",
			rancherVersion, version.MinRancherMinor)
	}
	if !version.SupportedRancher(rancher) {
		return fmt.Errorf("catalog.cattle.io/rancher-version %q is for Rancher %s: Rancher 2.%d and later are supported, 2.%d and before are not",
			rancherVersion, rancher, version.MinRancherMinor, version.MinRancherMinor-1)
	}
	return policy.CheckMajor(newVersion, rancherVersion)
}

// newEntries returns the names of the entries that value has and before does
// not. value is before with entries added at the end.
func newEntries(before, after string) ([]string, error) {
	type entry struct {
		Name string `yaml:"name"`
	}
	var old, now []entry
	if strings.TrimSpace(before) != "" {
		if err := yaml.Unmarshal([]byte(before), &old); err != nil {
			return nil, fmt.Errorf("read helm.sh/images: %w", err)
		}
	}
	if err := yaml.Unmarshal([]byte(after), &now); err != nil {
		return nil, fmt.Errorf("read helm.sh/images: %w", err)
	}

	names := make([]string, 0, len(now)-len(old))
	for _, e := range now[len(old):] {
		names = append(names, e.Name)
	}
	return names, nil
}
