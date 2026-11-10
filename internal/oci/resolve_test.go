package oci

import (
	"context"
	"net/http"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

func TestResolveDigest(t *testing.T) {
	f := newFakeRegistry()
	f.addChart(t, validArchive(t))
	manifest := f.responses["/v2/"+testRepository+"/manifests/"+testTag]

	got, err := ResolveDigest(context.Background(), serve(t, f), FetchOptions{PlainHTTP: true})

	require.NoError(t, err)
	require.Equal(t, digest.FromBytes(manifest.body).String(), got)
}

func TestResolveDigestFails(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, f *fakeRegistry)
		wantErr string
	}{
		{
			name:    "the tag does not exist",
			setup:   func(*testing.T, *fakeRegistry) {},
			wantErr: "was not found in the registry",
		},
		{
			name: "the registry asks for authentication",
			setup: func(_ *testing.T, f *fakeRegistry) {
				f.status["/v2/"+testRepository+"/manifests/"+testTag] = http.StatusUnauthorized
			},
			wantErr: "asks for authentication",
		},
		{
			name: "the registry forbids the read",
			setup: func(_ *testing.T, f *fakeRegistry) {
				f.status["/v2/"+testRepository+"/manifests/"+testTag] = http.StatusForbidden
			},
			wantErr: "asks for authentication",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRegistry()
			tt.setup(t, f)

			got, err := ResolveDigest(context.Background(), serve(t, f), FetchOptions{PlainHTTP: true})

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, got)
		})
	}
}

func TestResolveDigestCannotReachTheRegistry(t *testing.T) {
	f := newFakeRegistry()
	f.addChart(t, validArchive(t))
	srv := newServer(f)
	ref := ChartRef{Host: srv.Listener.Addr().String(), Repository: testRepository, Tag: testTag}
	srv.Close()

	got, err := ResolveDigest(context.Background(), ref, FetchOptions{PlainHTTP: true})

	require.ErrorContains(t, err, "fetch "+ref.String())
	require.Empty(t, got)
}
