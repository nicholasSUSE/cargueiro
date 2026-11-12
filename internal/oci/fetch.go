package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	helmregistry "helm.sh/helm/v4/pkg/registry"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"

	"github.com/rancher/cargueiro/internal/chart"
)

// What a registry may send is not trusted: the largest manifest and the
// largest chart layer that are read.
const (
	maxManifestSize = 1 << 20
	maxLayerSize    = 2 << 20
)

// FetchOptions tune how the registry is reached. Reads are anonymous unless a
// username and a password are given.
type FetchOptions struct {
	// Username and Password authenticate with basic auth. Both or neither.
	Username string
	Password string
	// PlainHTTP talks HTTP instead of HTTPS.
	PlainHTTP bool
}

// authenticated reports whether the reads carry credentials.
func (o FetchOptions) authenticated() bool {
	return o.Username != ""
}

// credential returns the basic-auth credential in o, or the empty credential
// for anonymous reads.
func (o FetchOptions) credential() auth.Credential {
	if !o.authenticated() {
		return auth.EmptyCredential
	}
	return auth.Credential{Username: o.Username, Password: o.Password}
}

// FetchChart reads Chart.yaml and values.yaml of the chart at ref. The chart
// layer is read in full and checked against the digest and the size the
// manifest gives for it, then only the two files are taken from it.
func FetchChart(ctx context.Context, ref ChartRef, opts FetchOptions) (*chart.Files, error) {
	repo, err := newRepository(ref, opts)
	if err != nil {
		return nil, err
	}

	layer, err := chartLayer(ctx, repo, ref.Tag, ref.String(), opts.authenticated())
	if err != nil {
		return nil, err
	}
	if layer.Size > maxLayerSize {
		return nil, fmt.Errorf("the chart layer of %s is %d bytes, larger than the %d bytes that are read", ref, layer.Size, maxLayerSize)
	}

	rc, err := repo.Fetch(ctx, layer)
	if err != nil {
		return nil, fetchError(ref.String(), err, opts.authenticated())
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

// newRepository returns the repository of ref, read with the credentials in
// opts, or anonymously without them.
func newRepository(ref ChartRef, opts FetchOptions) (*remote.Repository, error) {
	httpClient, err := newHTTPClient(nil)
	if err != nil {
		return nil, err
	}
	return openRepository(ref, httpClient, opts.credential(), opts.PlainHTTP)
}

// openRepository returns the repository of ref, reached through httpClient with
// credential.
func openRepository(ref ChartRef, httpClient *http.Client, credential auth.Credential, plainHTTP bool) (*remote.Repository, error) {
	repo, err := remote.NewRepository(ref.Host + "/" + ref.Repository)
	if err != nil {
		return nil, fmt.Errorf("invalid chart reference: %w", err)
	}

	authorizer := newAuthorizer(httpClient, credential)
	repo.Client = &authorizer
	repo.PlainHTTP = plainHTTP
	return repo, nil
}

// readManifest reads the manifest that reference, a tag or a digest, names in
// repo. what says what is read, for the errors.
func readManifest(ctx context.Context, repo *remote.Repository, reference, what string, authenticated bool) (ocispec.Manifest, error) {
	desc, rc, err := repo.FetchReference(ctx, reference)
	if err != nil {
		return ocispec.Manifest{}, fetchError(what, err, authenticated)
	}
	defer func() { _ = rc.Close() }()

	if desc.Size > maxManifestSize {
		return ocispec.Manifest{}, fmt.Errorf("the manifest of %s is %d bytes, larger than the %d bytes that are read", what, desc.Size, maxManifestSize)
	}
	data, err := content.ReadAll(rc, desc)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("read the manifest of %s: %w", what, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ocispec.Manifest{}, fmt.Errorf("%s is not a Helm chart: its manifest cannot be read: %w", what, err)
	}
	return manifest, nil
}

// chartLayer reads the manifest that reference, a tag or a digest, names in repo
// and returns the descriptor of its chart layer. The manifest must be a Helm
// chart's: the Helm config and one chart layer. A provenance layer next to it is
// ignored. what says what is read, for the errors.
func chartLayer(ctx context.Context, repo *remote.Repository, reference, what string, authenticated bool) (ocispec.Descriptor, error) {
	manifest, err := readManifest(ctx, repo, reference, what, authenticated)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if manifest.Config.MediaType != helmregistry.ConfigMediaType {
		return ocispec.Descriptor{}, fmt.Errorf("%s is not a Helm chart: its config is %q", what, manifest.Config.MediaType)
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
		return ocispec.Descriptor{}, fmt.Errorf("%s is not a Helm chart: it has no chart layer", what)
	default:
		return ocispec.Descriptor{}, fmt.Errorf("%s has more than one chart layer", what)
	}
}

// fetchError says what went wrong in terms of what was read, such as a
// reference. authenticated says whether the read carried credentials, which
// decides what a refusal means.
func fetchError(what string, err error, authenticated bool) error {
	var resp *errcode.ErrorResponse
	switch {
	case errors.Is(err, errdef.ErrNotFound):
		return fmt.Errorf("%s was not found in the registry", what)
	// A registry that asks for basic authentication, which needs credentials.
	case errors.Is(err, auth.ErrBasicCredentialNotFound),
		errors.As(err, &resp) && (resp.StatusCode == 401 || resp.StatusCode == 403):
		if authenticated {
			return fmt.Errorf("the registry refused the credentials for %s: check the username and the password, and that the account may read this repository", what)
		}
		return fmt.Errorf("the registry granted no access to %s without credentials: either the repository needs them, or it does not exist, which a registry answers in the same way. Give a username with --username and the password in $CARGUEIRO_PASSWORD, or save them with `cargueiro registry login`", what)
	default:
		return fmt.Errorf("fetch %s: %w", what, err)
	}
}
