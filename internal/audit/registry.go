package audit

import (
	"context"
	"strings"
	"sync"

	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/version"
)

// maxRequests is how many versions are read from the registry at the same time.
const maxRequests = 8

// FetchFunc reads the digests of the manifest that ref names.
type FetchFunc func(ctx context.Context, ref oci.ChartRef, opts oci.FetchOptions) (oci.ManifestInfo, error)

// remoteVersion is what the registry answered for a version: its manifest, or
// why it could not be read. A failed read is data, not a stop: the checks say
// what it means.
type remoteVersion struct {
	ref  oci.ChartRef
	info oci.ManifestInfo
	err  error
}

// gather reads the manifest of every version of the snapshot from the staging
// registry of its chart, with one request for each, at most maxRequests at a
// time. It reads where the rules say the version is, not where its url says, so
// a url that was edited to point somewhere else does not change what is read.
func gather(ctx context.Context, snap *snapshot, opts Options) {
	fetch := opts.Fetch
	if fetch == nil {
		fetch = oci.FetchManifestInfo
	}

	snap.remote = make(map[string]remoteVersion, len(snap.versions))
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		slot = make(chan struct{}, maxRequests)
	)
	for _, v := range snap.versions {
		wg.Add(1)
		slot <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slot }()

			got := readVersion(ctx, snap, v, opts, fetch)

			mu.Lock()
			snap.remote[versionName(v)] = got
			mu.Unlock()
		}()
	}
	wg.Wait()
}

// readVersion reads the manifest of one version.
func readVersion(ctx context.Context, snap *snapshot, v published, opts Options, fetch FetchFunc) remoteVersion {
	rule, err := snap.cfg.Find(v.chart)
	if err != nil {
		return remoteVersion{err: err}
	}
	repository, err := snap.cfg.StagingRepository(rule, snap.registry)
	if err != nil {
		return remoteVersion{err: err}
	}
	chartRepo, err := oci.ParseChartRepo(repository)
	if err != nil {
		return remoteVersion{err: err}
	}
	ref := oci.ChartRef{Host: chartRepo.Host, Repository: chartRepo.Repository, Tag: version.ToOCITag(v.entry.Version)}

	fetchOpts, err := opts.Credentials.FetchOptions(chartRepo.Host, opts.PlainHTTP)
	if err != nil {
		return remoteVersion{ref: ref, err: err}
	}
	info, err := fetch(ctx, ref, fetchOpts)
	return remoteVersion{ref: ref, info: info, err: err}
}

// pinnedDigest returns the digest that the url of the version pins, or "" when
// it has no single url with one.
func pinnedDigest(v published) string {
	if len(v.entry.URLs) != 1 {
		return ""
	}
	_, pinned, _ := strings.Cut(v.entry.URLs[0], "@")
	return pinned
}
