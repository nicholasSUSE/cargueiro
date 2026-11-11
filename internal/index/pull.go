package index

import (
	"context"
	"errors"
	"fmt"
	"strings"

	repo "helm.sh/helm/v4/pkg/repo/v1"

	"github.com/rancher/cargueiro/internal/oci"
)

// PullFunc returns the archive of the manifest that manifestDigest pins in repo.
type PullFunc func(ctx context.Context, repo oci.ChartRepo, manifestDigest string, opts oci.FetchOptions) ([]byte, error)

// pullAdded pulls the archive of each added version, from the manifest its URL
// pins, and checks that it is the chart its entry describes. It fails, naming
// every version at fault, when any is not, and returns no archive then.
func pullAdded(ctx context.Context, all *repo.IndexFile, added []Ref, opts Options) (map[Ref][]byte, error) {
	pull := opts.Pull
	if pull == nil {
		pull = oci.PullArchive
	}
	byRef := versionsByRef(all)

	archives := make(map[Ref][]byte, len(added))
	var problems []error
	for _, ref := range added {
		entry := byRef[ref]
		archive, err := pullOne(ctx, pull, entry, opts)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		archives[ref] = archive
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return archives, nil
}

// pullOne pulls the archive of the entry and checks it.
func pullOne(ctx context.Context, pull PullFunc, entry *repo.ChartVersion, opts Options) ([]byte, error) {
	chartRepo, manifestDigest, err := pinnedManifest(entry)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", entry.Name, entry.Version, err)
	}

	fetch, err := opts.Credentials.FetchOptions(chartRepo.Host, opts.PlainHTTP)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", entry.Name, entry.Version, err)
	}
	archive, err := pull(ctx, chartRepo, manifestDigest, fetch)
	if err != nil {
		return nil, fmt.Errorf("%s %s: pull the archive: %w", entry.Name, entry.Version, err)
	}
	if err := checkArchive(entry, archive); err != nil {
		return nil, err
	}
	return archive, nil
}

// pinnedManifest returns the repository and the digest of the manifest in the
// URL of the entry: oci://<repository>:<version>@<digest>.
func pinnedManifest(entry *repo.ChartVersion) (oci.ChartRepo, string, error) {
	if len(entry.URLs) != 1 {
		return oci.ChartRepo{}, "", fmt.Errorf("it has %d urls, and it needs exactly one", len(entry.URLs))
	}

	location, manifestDigest, found := strings.Cut(strings.TrimPrefix(entry.URLs[0], "oci://"), "@")
	if !found {
		return oci.ChartRepo{}, "", fmt.Errorf("the url %q pins no digest", entry.URLs[0])
	}
	chartRepo, err := oci.ParseChartRepo(strings.TrimSuffix(location, ":"+entry.Version))
	if err != nil {
		return oci.ChartRepo{}, "", fmt.Errorf("the url %q: %w", entry.URLs[0], err)
	}
	return chartRepo, manifestDigest, nil
}
