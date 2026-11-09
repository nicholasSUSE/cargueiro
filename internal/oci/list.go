package oci

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"oras.land/oras-go/v2/registry/remote/errcode"
)

// ErrRepositoryNotFound is wrapped by the error ListTags returns for a
// repository that the registry does not have. A chart that was never pushed is
// in this case.
var ErrRepositoryNotFound = errors.New("repository not found")

// maxTags is the most tags of one chart that are read. What a registry sends is
// not trusted.
const maxTags = 100_000

// ListTags returns every tag of the chart at repo, in the order the registry
// gives them. It follows the registry's own pages. A repository with no tags
// gives none. Reads are anonymous.
func ListTags(ctx context.Context, repo ChartRepo, opts FetchOptions) ([]string, error) {
	return listTags(ctx, repo, opts, maxTags)
}

// listTags is ListTags with the most tags it reads.
func listTags(ctx context.Context, repo ChartRepo, opts FetchOptions, limit int) ([]string, error) {
	remoteRepo, err := newRepository(ChartRef{Host: repo.Host, Repository: repo.Repository}, opts)
	if err != nil {
		return nil, err
	}

	var tags []string
	err = remoteRepo.Tags(ctx, "", func(page []string) error {
		if len(tags)+len(page) > limit {
			return fmt.Errorf("%s has more than %d tags", repo, limit)
		}
		tags = append(tags, page...)
		return nil
	})
	var resp *errcode.ErrorResponse
	switch {
	case errors.As(err, &resp) && resp.StatusCode == http.StatusNotFound:
		// A tag list has no not-found error of its own: the registry answers 404.
		return nil, fmt.Errorf("%s was not found in the registry: %w", repo, ErrRepositoryNotFound)
	case err != nil:
		return nil, fetchError(repo.String(), err)
	}
	return tags, nil
}
