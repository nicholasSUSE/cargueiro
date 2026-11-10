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

	"github.com/nicholasSUSE/cargueiro/internal/annotations"
	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
	"github.com/nicholasSUSE/cargueiro/internal/version"
)

// helmImages is the annotation that lists the images of a chart.
const helmImages = "helm.sh/images"

// ListFunc returns the tags of the chart at repo.
type ListFunc func(ctx context.Context, repo oci.ChartRepo, opts oci.FetchOptions) ([]string, error)

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
	// List reads the tags of the registry. It is only used to work out the
	// version.
	List ListFunc
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
	if newVersion == "" {
		if newVersion, err = nextVersion(ctx, cfg, rule, files, opts); err != nil {
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

	if _, err := supportedRancher(files); err != nil {
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

// nextVersion works out the version of the next release of the chart from the
// versions already released: the major comes from the rancher-version of the
// chart, the upstream part from its appVersion, and the release counter from
// what the registry has. A chart that was never pushed starts at the first
// version.
func nextVersion(ctx context.Context, cfg *rules.Config, rule rules.Chart, files *chart.Files, opts Options) (string, error) {
	if rule.Kind == rules.KindCRD {
		return "", errors.New("a CRD chart has no rancher-version, so stamp cannot work out its version: give --version")
	}
	rancher, err := supportedRancher(files)
	if err != nil {
		return "", err
	}
	versions := cfg.Versions()
	major, ok := versions.Majors[rancher]
	if !ok {
		return "", fmt.Errorf("the rules file has no chart major for Rancher %s", rancher)
	}
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return "", err
	}

	upstream := strings.TrimPrefix(files.Metadata.AppVersion, "v")
	if !rule.PlainVersion && upstream == "" {
		return "", errors.New("the Chart.yaml has no appVersion, which stamp needs for the upstream part of the version: give --version")
	}

	published, err := published(ctx, cfg, rule, opts)
	if err != nil {
		return "", err
	}
	next, err := policy.Next(published, major, upstream, rule.PlainVersion)
	if err != nil {
		return "", fmt.Errorf("%w: give --version to set it by hand", err)
	}
	return next, nil
}

// published returns the versions of the chart that are already released. A
// repository that does not exist has none.
func published(ctx context.Context, cfg *rules.Config, rule rules.Chart, opts Options) ([]string, error) {
	reference, err := cfg.StagingRepository(rule, opts.Registry)
	if err != nil {
		return nil, err
	}
	repo, err := oci.ParseChartRepo(reference)
	if err != nil {
		return nil, err
	}

	tags, err := opts.List(ctx, repo, oci.FetchOptions{PlainHTTP: opts.PlainHTTP})
	if err != nil {
		if errors.Is(err, oci.ErrRepositoryNotFound) {
			return nil, nil
		}
		return nil, err
	}

	versions := make([]string, len(tags))
	for i, tag := range tags {
		versions[i] = version.FromOCITag(tag)
	}
	return versions, nil
}

// supportedRancher returns the Rancher version of the lower bound of the
// rancher-version annotation of the chart, such as 2.16. It fails when the
// annotation has none, or when it is a Rancher version the new system does not
// support.
func supportedRancher(files *chart.Files) (string, error) {
	rancherVersion := annotations.RancherVersionAnnotation(files.Annotations)
	rancher, ok := version.RancherOf(rancherVersion)
	if !ok {
		return "", fmt.Errorf("catalog.cattle.io/rancher-version %q has no lower bound like \">= 2.%d.0-0\": stamp needs it to know the major of the version",
			rancherVersion, version.MinRancherMinor)
	}
	if !version.SupportedRancher(rancher) {
		return "", fmt.Errorf("catalog.cattle.io/rancher-version %q is for Rancher %s: Rancher 2.%d and later are supported, 2.%d and before are not",
			rancherVersion, rancher, version.MinRancherMinor, version.MinRancherMinor-1)
	}
	return rancher, nil
}
