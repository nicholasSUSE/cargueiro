package oci

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

func TestFetchManifestInfo(t *testing.T) {
	archive := validArchive(t)
	f := newFakeRegistry()
	layer := f.addChart(t, archive)
	pinned := f.pin(t)

	got, err := FetchManifestInfo(context.Background(), serve(t, f), FetchOptions{PlainHTTP: true})

	require.NoError(t, err)
	require.Equal(t, ManifestInfo{ManifestDigest: pinned, ChartDigest: layer.Digest.String()}, got)
	require.Equal(t, digest.FromBytes(archive).String(), got.ChartDigest)
	require.False(t, f.wasRequested(blobPath(layer)), "the chart layer is not downloaded")
}

func TestFetchManifestInfoFails(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, f *fakeRegistry)
		wantErr string
		missing bool
	}{
		{
			name:    "the tag does not exist",
			setup:   func(*testing.T, *fakeRegistry) {},
			wantErr: "was not found in the registry",
			missing: true,
		},
		{
			name: "the registry answers a manifest with another digest",
			setup: func(t *testing.T, f *fakeRegistry) {
				f.addChart(t, validArchive(t))
				path := "/v2/" + testRepository + "/manifests/" + testTag
				r := f.responses[path]
				r.digest = digest.FromString("another manifest").String()
				f.responses[path] = r
			},
			wantErr: "read the manifest of",
		},
		{
			name: "the registry refuses the read",
			setup: func(_ *testing.T, f *fakeRegistry) {
				f.status["/v2/"+testRepository+"/manifests/"+testTag] = http.StatusUnauthorized
			},
			wantErr: "granted no access",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRegistry()
			tt.setup(t, f)

			_, err := FetchManifestInfo(context.Background(), serve(t, f), FetchOptions{PlainHTTP: true})

			require.ErrorContains(t, err, tt.wantErr)
			require.Equal(t, tt.missing, errors.Is(err, ErrNotFound))
		})
	}
}
