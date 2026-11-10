package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// tagRegistry serves the tag list of one repository, in pages.
type tagRegistry struct {
	repository string
	tags       []string
	pageSize   int
	// status, when it is not 0, is answered for the tag list instead of it.
	status int
	// pages counts the tag list requests.
	pages int
}

func (f *tagRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v2/" {
		return
	}
	if r.URL.Path != "/v2/"+f.repository+"/tags/list" {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"code":"NAME_UNKNOWN","message":"repository name not known to registry"}]}`)
		return
	}

	f.pages++
	if f.status != 0 {
		w.WriteHeader(f.status)
		return
	}

	start := 0
	if last := r.URL.Query().Get("last"); last != "" {
		for i, tag := range f.tags {
			if tag == last {
				start = i + 1
			}
		}
	}
	end := len(f.tags)
	if f.pageSize > 0 && start+f.pageSize < end {
		end = start + f.pageSize
		next := url.Values{"last": {f.tags[end-1]}}
		w.Header().Set("Link", fmt.Sprintf(`</v2/%s/tags/list?%s>; rel="next"`, f.repository, next.Encode()))
	}

	page := f.tags[start:end]
	body := map[string]any{"name": f.repository, "tags": page}
	if len(page) == 0 {
		body["tags"] = nil
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// serveTags starts a registry and returns the repository it serves.
func serveTags(t *testing.T, f *tagRegistry) ChartRepo {
	t.Helper()

	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return ChartRepo{Host: strings.TrimPrefix(srv.URL, "http://"), Repository: f.repository}
}

func TestListTags(t *testing.T) {
	tests := []struct {
		name      string
		tags      []string
		pageSize  int
		want      []string
		wantPages int
	}{
		{
			name:      "one page",
			tags:      []string{"110.0.3_up0.16.3", "110.0.4_up0.16.4"},
			pageSize:  0,
			want:      []string{"110.0.3_up0.16.3", "110.0.4_up0.16.4"},
			wantPages: 1,
		},
		{
			name:      "many pages",
			tags:      []string{"a1", "a2", "a3", "a4", "a5"},
			pageSize:  2,
			want:      []string{"a1", "a2", "a3", "a4", "a5"},
			wantPages: 3,
		},
		{
			name:      "the last page is full",
			tags:      []string{"a1", "a2", "a3", "a4"},
			pageSize:  2,
			want:      []string{"a1", "a2", "a3", "a4"},
			wantPages: 2,
		},
		{
			name:      "no tags",
			tags:      nil,
			pageSize:  0,
			want:      nil,
			wantPages: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &tagRegistry{repository: "rancher/fleet", tags: tt.tags, pageSize: tt.pageSize}
			repo := serveTags(t, f)

			got, err := ListTags(context.Background(), repo, FetchOptions{PlainHTTP: true})

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.wantPages, f.pages)
		})
	}
}

func TestListTagsFails(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		repo    string
		wantErr string
	}{
		{
			name:    "a repository that does not exist",
			repo:    "rancher/other",
			wantErr: "was not found in the registry",
		},
		{
			name:    "the registry asks for credentials",
			status:  http.StatusUnauthorized,
			wantErr: "the registry granted no access",
		},
		{
			name:    "the registry refuses",
			status:  http.StatusForbidden,
			wantErr: "the registry granted no access",
		},
		{
			name:    "the registry fails",
			status:  http.StatusInternalServerError,
			wantErr: "fetch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &tagRegistry{repository: "rancher/fleet", tags: []string{"a"}, status: tt.status}
			repo := serveTags(t, f)
			if tt.repo != "" {
				repo.Repository = tt.repo
			}

			got, err := ListTags(context.Background(), repo, FetchOptions{PlainHTTP: true})

			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, repo.String())
			require.Equal(t, tt.repo != "", errors.Is(err, ErrRepositoryNotFound),
				"only a repository that does not exist is ErrRepositoryNotFound")
			require.Nil(t, got)
		})
	}
}

func TestListTagsStopsAtTheLimit(t *testing.T) {
	f := &tagRegistry{repository: "rancher/fleet", tags: []string{"a1", "a2", "a3", "a4", "a5"}, pageSize: 2}
	repo := serveTags(t, f)

	got, err := listTags(context.Background(), repo, FetchOptions{PlainHTTP: true}, 4)

	require.ErrorContains(t, err, "has more than 4 tags")
	require.Nil(t, got)
	require.Equal(t, 3, f.pages)
}

func TestListTagsCannotReachTheRegistry(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	repo := ChartRepo{Host: strings.TrimPrefix(srv.URL, "http://"), Repository: "rancher/fleet"}
	srv.Close()

	got, err := ListTags(context.Background(), repo, FetchOptions{PlainHTTP: true})

	require.ErrorContains(t, err, "fetch "+repo.String())
	require.Nil(t, got)
}

func TestListTagsOverHTTPSNeedsHTTPS(t *testing.T) {
	f := &tagRegistry{repository: "rancher/fleet", tags: []string{"a"}}
	repo := serveTags(t, f)

	got, err := ListTags(context.Background(), repo, FetchOptions{PlainHTTP: false})

	require.Error(t, err)
	require.Nil(t, got)
	require.Zero(t, f.pages)
}
