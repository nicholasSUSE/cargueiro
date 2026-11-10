package oci

import (
	"context"
)

// ResolveDigest returns the digest of the manifest that the tag of ref names. It
// does not read the manifest: it is how a tag is turned into the digest that pins
// it. Reads are anonymous.
func ResolveDigest(ctx context.Context, ref ChartRef, opts FetchOptions) (string, error) {
	repo, err := newRepository(ref, opts)
	if err != nil {
		return "", err
	}

	desc, err := repo.Resolve(ctx, ref.Tag)
	if err != nil {
		return "", fetchError(ref.String(), err)
	}
	return desc.Digest.String(), nil
}
