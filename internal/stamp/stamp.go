// Package stamp puts the marks Rancher needs on a chart directory: the final
// version and the helm.sh/images annotation. It validates before it changes
// anything, and changes only those two places of Chart.yaml.
package stamp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/rancher/cargueiro/internal/annotations"
	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/chartversion"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/version"
)

// helmImages is the annotation that lists the images of a chart.
const helmImages = "helm.sh/images"

// ListFunc returns the tags of the chart at repo.
type ListFunc = chartversion.ListFunc

// Options say how to stamp a chart.
type Options struct {
	// Version is the final version of the chart. It is not compared with
	// anything but the version rules: whoever gives it owns it. When it is empty
	// the version is worked out from what is already released.
	Version string
	// Registry replaces the registry of the chart's edition, as host[:port]/path.
	// The name of the chart is added to it. It is only used to work out the
	// version.
	Registry string
	// PlainHTTP talks HTTP to that registry.
	PlainHTTP bool
	// Credentials gives the credentials to read from that registry. Without it
	// the read is anonymous.
	Credentials oci.CredentialFunc
	// List reads the tags of the registry. It is only used to work out the
	// version.
	List ListFunc
}

// versionOptions are the options to work out the version.
func (o Options) versionOptions() chartversion.Options {
	return chartversion.Options{Registry: o.Registry, PlainHTTP: o.PlainHTTP, Credentials: o.Credentials, List: o.List}
}

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
	// Released are the versions the registry had when the version was worked
	// out. It is empty when the version was given.
	Released []string
}

// Changed reports whether stamping changes Chart.yaml.
func (p *Plan) Changed() bool {
	return !bytes.Equal(p.Before, p.After)
}

// Build works out what stamping dir would change. The chart must be listed in
// cfg. It fails, and says why, before anything is written, when the version does
// not follow the rules, when the chart is for a Rancher version the new system
// does not support, or when an image of values.yaml cannot be listed in
// helm.sh/images.
func Build(ctx context.Context, cfg *rules.Config, dir string, opts Options) (*Plan, error) {
	files, err := chart.Load(dir)
	if err != nil {
		return nil, err
	}
	rule, err := cfg.Find(files.Metadata.Name)
	if err != nil {
		return nil, err
	}
	newVersion := opts.Version
	var released []string
	if newVersion == "" {
		if newVersion, released, err = chartversion.Next(ctx, cfg, rule, files, opts.versionOptions()); err != nil {
			return nil, err
		}
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
		Released:    released,
	}, nil
}

// Write replaces Chart.yaml with the planned one, through a temporary file in
// the same directory, so that it is never left half written. It does nothing
// when nothing changes.
func (p *Plan) Write() error {
	if !p.Changed() {
		return nil
	}
	return replaceFile(p.Path, p.After)
}

// restore puts the Chart.yaml that was there before back.
func (p *Plan) restore() error {
	return replaceFile(p.Path, p.Before)
}

// replaceFile replaces the file at path with data, through a temporary file in
// the same directory, and keeps its permissions.
func replaceFile(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".Chart.yaml.*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone after the rename; this is for a failure

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Chmod(tmp.Name(), info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
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

	if _, err := chartversion.SupportedRancher(files); err != nil {
		return err
	}
	return policy.CheckMajor(newVersion, annotations.RancherVersionAnnotation(files.Annotations))
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
