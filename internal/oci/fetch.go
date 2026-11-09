package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	helmregistry "helm.sh/helm/v4/pkg/registry"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
)

// What a registry may send is not trusted: the largest manifest and the
// largest chart layer that are read.
const (
	maxManifestSize = 1 << 20
	maxLayerSize    = 2 << 20
)

// FetchOptions tune how the registry is reached. Reads are anonymous.
type FetchOptions struct {
	// PlainHTTP talks HTTP instead of HTTPS.
	PlainHTTP bool
}

// FetchChart reads Chart.yaml and values.yaml of the chart at ref. The chart
// layer is read in full and checked against the digest and the size the
// manifest gives for it, then only the two files are taken from it.
func FetchChart(ctx context.Context, ref ChartRef, opts FetchOptions) (*chart.Files, error) {
	repo, err := newRepository(ref, opts)
	if err != nil {
		return nil, err
	}

	layer, err := chartLayer(ctx, repo, ref)
	if err != nil {
		return nil, err
	}
	if layer.Size > maxLayerSize {
		return nil, fmt.Errorf("the chart layer of %s is %d bytes, larger than the %d bytes that are read", ref, layer.Size, maxLayerSize)
	}

	rc, err := repo.Fetch(ctx, layer)
	if err != nil {
		return nil, fetchError(ref, err)
	}
	defer func() { _ = rc.Close() }()

	data, err := content.ReadAll(rc, layer)
	if err != nil {
		return nil, fmt.Errorf("read the chart layer of %s: %w", ref, err)
	}

	files, err := chart.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("read the chart in %s: %w", ref, err)
	}
	return files, nil
}

// newRepository returns the repository of ref, read without credentials.
func newRepository(ref ChartRef, opts FetchOptions) (*remote.Repository, error) {
	repo, err := remote.NewRepository(ref.Host + "/" + ref.Repository)
	if err != nil {
		return nil, fmt.Errorf("invalid chart reference: %w", err)
	}

	httpClient, err := newHTTPClient(nil)
	if err != nil {
		return nil, err
	}
	authorizer := newAuthorizer(httpClient, auth.EmptyCredential)
	repo.Client = &authorizer
	repo.PlainHTTP = opts.PlainHTTP
	return repo, nil
}

// chartLayer reads the manifest of ref and returns the descriptor of its chart
// layer. The manifest must be a Helm chart's: the Helm config and one chart
// layer. A provenance layer next to it is ignored.
func chartLayer(ctx context.Context, repo *remote.Repository, ref ChartRef) (ocispec.Descriptor, error) {
	desc, rc, err := repo.FetchReference(ctx, ref.Tag)
	if err != nil {
		return ocispec.Descriptor{}, fetchError(ref, err)
	}
	defer func() { _ = rc.Close() }()

	if desc.Size > maxManifestSize {
		return ocispec.Descriptor{}, fmt.Errorf("the manifest of %s is %d bytes, larger than the %d bytes that are read", ref, desc.Size, maxManifestSize)
	}
	data, err := content.ReadAll(rc, desc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("read the manifest of %s: %w", ref, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("%s is not a Helm chart: its manifest cannot be read: %w", ref, err)
	}
	if manifest.Config.MediaType != helmregistry.ConfigMediaType {
		return ocispec.Descriptor{}, fmt.Errorf("%s is not a Helm chart: its config is %q", ref, manifest.Config.MediaType)
	}

	var layers []ocispec.Descriptor
	for _, l := range manifest.Layers {
		if l.MediaType == helmregistry.ChartLayerMediaType {
			layers = append(layers, l)
		}
	}
	switch len(layers) {
	case 1:
		return layers[0], nil
	case 0:
		return ocispec.Descriptor{}, fmt.Errorf("%s is not a Helm chart: it has no chart layer", ref)
	default:
		return ocispec.Descriptor{}, fmt.Errorf("%s has more than one chart layer", ref)
	}
}

// fetchError says what went wrong in terms of the reference.
func fetchError(ref ChartRef, err error) error {
	var resp *errcode.ErrorResponse
	switch {
	case errors.Is(err, errdef.ErrNotFound):
		return fmt.Errorf("%s was not found in the registry", ref)
	case errors.As(err, &resp) && (resp.StatusCode == 401 || resp.StatusCode == 403):
		return fmt.Errorf("the registry asks for authentication to read %s, which is not supported yet", ref)
	default:
		return fmt.Errorf("fetch %s: %w", ref, err)
	}
}
