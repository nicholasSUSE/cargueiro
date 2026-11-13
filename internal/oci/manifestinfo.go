package oci

import (
	"context"
)

// ManifestInfo is what a registry holds for a tag of a chart.
type ManifestInfo struct {
	// ManifestDigest is the digest of the manifest the tag names, as
	// "sha256:<hex>". It is the digest of the bytes that were read.
	ManifestDigest string
	// ChartDigest is the digest the manifest gives for its chart layer, which
	// is the sha256 of the chart archive, as "sha256:<hex>".
	ChartDigest string
}

// FetchManifestInfo reads the manifest that ref names with one request and
// returns its digest and the digest of its chart layer. The chart layer itself
// is not downloaded.
//
// A tag the registry does not have is an error that matches ErrNotFound.
func FetchManifestInfo(ctx context.Context, ref ChartRef, opts FetchOptions) (ManifestInfo, error) {
	repo, err := newRepository(ref, opts)
	if err != nil {
		return ManifestInfo{}, err
	}

	desc, manifest, err := readManifest(ctx, repo, ref.Tag, ref.String(), opts.authenticated())
	if err != nil {
		return ManifestInfo{}, err
	}
	layer, err := layerOf(manifest, ref.String())
	if err != nil {
		return ManifestInfo{}, err
	}
	return ManifestInfo{ManifestDigest: desc.Digest.String(), ChartDigest: layer.Digest.String()}, nil
}
