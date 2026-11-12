// Package chartversion works out the final version of a chart, or of a family,
// from the Rancher version and the appVersion of its Chart.yaml and the versions
// already released in the staging registry of SUSE.
package chartversion

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/cargueiro/internal/annotations"
	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/version"
)

// ListFunc returns the tags of the chart at repo.
type ListFunc func(ctx context.Context, repo oci.ChartRepo, opts oci.FetchOptions) ([]string, error)

// Options say where the released versions are read from.
type Options struct {
	// Registry replaces the registry of the chart's edition, as host[:port]/path.
	// The name of the chart is added to it.
	Registry string
	// PlainHTTP talks HTTP to that registry.
	PlainHTTP bool
	// Credentials gives the credentials to read from that registry. Without it
	// the read is anonymous.
	Credentials oci.CredentialFunc
	// List reads the tags of the registry.
	List ListFunc
}

// Next works out the version of the next release of the chart from the
// versions already released: the major comes from the rancher-version of the
// chart, the upstream part from its appVersion, and the release counter from
// what the registry has, which is returned too. A chart that was never pushed
// starts at the first version.
func Next(ctx context.Context, cfg *rules.Config, rule rules.Chart, files *chart.Files, opts Options) (string, []string, error) {
	if rule.Kind == rules.KindCRD {
		return "", nil, errors.New("a CRD chart has no rancher-version, so its version cannot be worked out on its own")
	}
	rancher, err := SupportedRancher(files)
	if err != nil {
		return "", nil, err
	}
	versions := cfg.Versions()
	major, ok := versions.Majors[rancher]
	if !ok {
		return "", nil, fmt.Errorf("the rules file has no chart major for Rancher %s", rancher)
	}
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return "", nil, err
	}

	upstream := strings.TrimPrefix(files.Metadata.AppVersion, "v")
	if !rule.PlainVersion && upstream == "" {
		return "", nil, errors.New("the Chart.yaml has no appVersion, which the upstream part of the version needs")
	}

	published, err := published(ctx, cfg, rule, opts)
	if err != nil {
		return "", nil, err
	}
	next, err := policy.Next(published, major, upstream, rule.PlainVersion)
	if err != nil {
		return "", nil, err
	}
	return next, published, nil
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

	fetch, err := opts.Credentials.FetchOptions(repo.Host, opts.PlainHTTP)
	if err != nil {
		return nil, err
	}
	tags, err := opts.List(ctx, repo, fetch)
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

// SupportedRancher returns the Rancher version of the lower bound of the
// rancher-version annotation of the chart, such as 2.16. It fails when the
// annotation has none, or when it is a Rancher version the new system does not
// support.
func SupportedRancher(files *chart.Files) (string, error) {
	rancherVersion := annotations.RancherVersionAnnotation(files.Annotations)
	rancher, ok := version.RancherOf(rancherVersion)
	if !ok {
		return "", fmt.Errorf("catalog.cattle.io/rancher-version %q has no lower bound like \">= 2.%d.0-0\": the major of the version comes from it",
			rancherVersion, version.MinRancherMinor)
	}
	if !version.SupportedRancher(rancher) {
		return "", fmt.Errorf("catalog.cattle.io/rancher-version %q is for Rancher %s: Rancher 2.%d and later are supported, 2.%d and before are not",
			rancherVersion, rancher, version.MinRancherMinor, version.MinRancherMinor-1)
	}
	return rancher, nil
}

// CheckReleased fails unless the latest version released by each chart of the
// family is the one the main chart has, whose released versions are given. A
// chart with no repository yet has no versions. The way out is to give the
// version by hand.
func CheckReleased(ctx context.Context, cfg *rules.Config, members rules.Family, mainReleased []string, opts Options) error {
	versions := cfg.Versions()
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return err
	}
	latest := func(chart rules.Chart, released []string) (string, error) {
		if len(released) == 0 {
			return "", nil
		}
		sorted, err := policy.SortNewestFirst(released, chart.PlainVersion)
		if err != nil {
			return "", fmt.Errorf("%s: %w", chart.Name, err)
		}
		return sorted[0], nil
	}

	main := members.Main()
	want, err := latest(main, mainReleased)
	if err != nil {
		return err
	}
	described := []string{describeLatest(main.Name, want)}

	var problems []error
	differ := false
	for _, chart := range members.Charts[1:] {
		released, err := published(ctx, cfg, chart, opts)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", chart.Name, err))
			continue
		}
		got, err := latest(chart, released)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		described = append(described, describeLatest(chart.Name, got))
		differ = differ || got != want
	}
	if differ {
		problems = append(problems, fmt.Errorf("the charts of the family %q are not at the same released version: %s",
			members.Name, strings.Join(described, ", ")))
	}
	return errors.Join(problems...)
}

func describeLatest(chart, latest string) string {
	if latest == "" {
		return chart + " (nothing released)"
	}
	return chart + " " + latest
}
