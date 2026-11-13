package audit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/oci"
)

const (
	manifestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	manifestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	archiveC  = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	archiveD  = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
)

// fleetVersion is a version of the fleet chart in an index.yaml: a manifest
// digest pinned by its url, and the digest of its archive.
func fleetVersion(version, pinned, archive string) string {
	tag := strings.ReplaceAll(version, "+", "_")
	return "  - apiVersion: v2\n    name: fleet\n    version: " + version +
		"\n    digest: " + archive +
		"\n    urls:\n    - oci://stgregistry.suse.com/rancher/catalog/fleet:" + tag + "@" + pinned + "\n"
}

// fleetIndex is an index.yaml with the versions of the fleet chart given.
func fleetIndex(versions ...string) string {
	return "apiVersion: v1\nentries:\n  fleet:\n" + strings.Join(versions, "")
}

// registryStub is a registry that answers from memory, and records what it was asked.
type registryStub struct {
	mu       sync.Mutex
	tags     map[string]oci.ManifestInfo
	requests []string
	active   int
	peak     int
	delay    time.Duration
	// failWith is what every other read gets, when set.
	failWith error
}

// fetch is the FetchFunc of the stub, which keys its tags by "repository:tag".
func (r *registryStub) fetch(_ context.Context, ref oci.ChartRef, _ oci.FetchOptions) (oci.ManifestInfo, error) {
	r.mu.Lock()
	r.requests = append(r.requests, ref.Host+"/"+ref.Repository+":"+ref.Tag)
	r.active++
	r.peak = max(r.peak, r.active)
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.active--
		r.mu.Unlock()
	}()

	time.Sleep(r.delay)
	if r.failWith != nil {
		return oci.ManifestInfo{}, r.failWith
	}
	info, found := r.tags[ref.Host+"/"+ref.Repository+":"+ref.Tag]
	if !found {
		return oci.ManifestInfo{}, fmt.Errorf("%s was not found in the registry: %w", ref, oci.ErrNotFound)
	}
	return info, nil
}

func TestRunAsksTheRegistryOnceForEachVersion(t *testing.T) {
	var versions []string
	stub := &registryStub{tags: map[string]oci.ManifestInfo{}, delay: 5 * time.Millisecond}
	for i := range 30 {
		v := fmt.Sprintf("111.0.%d+up0.17.%d", i, i)
		versions = append(versions, fleetVersion(v, manifestA, archiveC))
		stub.tags[fmt.Sprintf("stgregistry.suse.com/rancher/catalog/fleet:111.0.%d_up0.17.%d", i, i)] = oci.ManifestInfo{ManifestDigest: manifestA, ChartDigest: "sha256:" + archiveC}
	}

	report := run(t, Options{Dir: writeIndex(t, fleetIndex(versions...)), Fetch: stub.fetch})

	require.True(t, report.Valid(), report)
	require.Len(t, stub.requests, 30, "the two checks share one read of each version")
	require.LessOrEqual(t, stub.peak, maxRequests)
	require.Greater(t, stub.peak, 1, "versions are read in parallel")
}

func TestRunReadsWhereTheRulesSayAndNotWhereTheUrlSays(t *testing.T) {
	stub := &registryStub{tags: map[string]oci.ManifestInfo{}}
	text := "apiVersion: v1\nentries:\n  fleet:\n  - apiVersion: v2\n    name: fleet\n    version: 111.0.3+up0.17.3\n    digest: " + archiveC +
		"\n    urls:\n    - oci://evil.example.com/fleet:111.0.3_up0.17.3@" + manifestA + "\n"

	run(t, Options{Dir: writeIndex(t, text), Fetch: stub.fetch})

	require.Equal(t, []string{"stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3"}, stub.requests)
}

func TestRunReadsTheRegistryGiven(t *testing.T) {
	stub := &registryStub{tags: map[string]oci.ManifestInfo{}}
	dir := writeIndex(t, fleetIndex(fleetVersion("111.0.3+up0.17.3", manifestA, archiveC)))

	run(t, Options{Dir: dir, Registry: "127.0.0.1:5052/e2e", Fetch: stub.fetch})

	require.Equal(t, []string{"127.0.0.1:5052/e2e/fleet:111.0.3_up0.17.3"}, stub.requests)
}

func TestRunGivesTheCredentialsOfTheRegistry(t *testing.T) {
	var host string
	creds := func(h string) (string, string, error) {
		host = h
		return "user", "secret", nil
	}
	var got oci.FetchOptions
	fetch := func(_ context.Context, _ oci.ChartRef, opts oci.FetchOptions) (oci.ManifestInfo, error) {
		got = opts
		return oci.ManifestInfo{}, nil
	}
	dir := writeIndex(t, fleetIndex(fleetVersion("111.0.3+up0.17.3", manifestA, archiveC)))

	run(t, Options{Dir: dir, PlainHTTP: true, Credentials: creds, Fetch: fetch})

	require.Equal(t, "stgregistry.suse.com", host)
	require.Equal(t, oci.FetchOptions{Username: "user", Password: "secret", PlainHTTP: true}, got)
}
