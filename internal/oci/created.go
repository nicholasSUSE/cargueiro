package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ManifestCreated returns when the chart of the manifest that manifestDigest
// pins in repo was pushed, in UTC. Helm records it in the annotation
// org.opencontainers.image.created of the manifest when it pushes, so it is the
// same every time it is read. A manifest without the annotation, or with one
// that is not a time, is an error: there is no other date to take instead that
// would be the same twice.
//
// The manifest is read by its digest, so the registry cannot answer with
// another one.
func ManifestCreated(ctx context.Context, repo ChartRepo, manifestDigest string, opts FetchOptions) (time.Time, error) {
	if d, err := digest.Parse(manifestDigest); err != nil || d.Algorithm() != digest.SHA256 {
		return time.Time{}, fmt.Errorf("%q is not a sha256 digest", manifestDigest)
	}
	what := repo.String() + "@" + manifestDigest

	remote, err := newRepository(ChartRef{Host: repo.Host, Repository: repo.Repository}, opts)
	if err != nil {
		return time.Time{}, err
	}
	_, manifest, err := readManifest(ctx, remote, manifestDigest, what, opts.authenticated())
	if err != nil {
		return time.Time{}, err
	}

	value, found := manifest.Annotations[ocispec.AnnotationCreated]
	if !found {
		return time.Time{}, fmt.Errorf("the manifest of %s has no %s annotation, which says when the chart was pushed", what, ocispec.AnnotationCreated)
	}
	created, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("the %s annotation of the manifest of %s is %q, which is not a time like 2006-01-02T15:04:05Z", ocispec.AnnotationCreated, what, value)
	}
	return created.UTC(), nil
}
