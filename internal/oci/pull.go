package oci

import (
	"context"
	"fmt"

	"github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2/content"
)

// PullArchive returns the chart archive (.tgz) of the manifest that
// manifestDigest pins in repo. The archive is not opened.
//
// The registry cannot answer with another manifest or other bytes: the manifest
// is read by its digest, and the chart layer is checked against the digest and
// the size the manifest gives for it. Reads are anonymous.
func PullArchive(ctx context.Context, repo ChartRepo, manifestDigest string, opts FetchOptions) ([]byte, error) {
	if d, err := digest.Parse(manifestDigest); err != nil || d.Algorithm() != digest.SHA256 {
		return nil, fmt.Errorf("%q is not a sha256 digest", manifestDigest)
	}
	what := repo.String() + "@" + manifestDigest

	remote, err := newRepository(ChartRef{Host: repo.Host, Repository: repo.Repository}, opts)
	if err != nil {
		return nil, err
	}

	layer, err := chartLayer(ctx, remote, manifestDigest, what, opts.authenticated())
	if err != nil {
		return nil, err
	}
	if layer.Size > maxLayerSize {
		return nil, fmt.Errorf("the chart layer of %s is %d bytes, larger than the %d bytes that are read", what, layer.Size, maxLayerSize)
	}

	rc, err := remote.Fetch(ctx, layer)
	if err != nil {
		return nil, fetchError(what, err, opts.authenticated())
	}
	defer func() { _ = rc.Close() }()

	data, err := content.ReadAll(rc, layer)
	if err != nil {
		return nil, fmt.Errorf("read the chart layer of %s: %w", what, err)
	}
	return data, nil
}
