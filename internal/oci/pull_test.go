package oci

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	helmregistry "helm.sh/helm/v4/pkg/registry"
)

// pin serves the manifest stored under testTag at its digest too, and returns
// the digest.
func (f *fakeRegistry) pin(t *testing.T) string {
	t.Helper()

	manifest, found := f.responses["/v2/"+testRepository+"/manifests/"+testTag]
	require.True(t, found, "there is no manifest to pin")
	pinned := digest.FromBytes(manifest.body).String()
	f.responses["/v2/"+testRepository+"/manifests/"+pinned] = manifest
	return pinned
}

// serveRepo starts f and returns the repository of the test chart on it.
func serveRepo(t *testing.T, f *fakeRegistry) ChartRepo {
	t.Helper()

	ref := serve(t, f)
	return ChartRepo{Host: ref.Host, Repository: ref.Repository}
}

func TestPullArchive(t *testing.T) {
	archive := validArchive(t)
	f := newFakeRegistry()
	f.addChart(t, archive)
	pinned := f.pin(t)

	got, err := PullArchive(context.Background(), serveRepo(t, f), pinned, FetchOptions{PlainHTTP: true})

	require.NoError(t, err)
	require.Equal(t, archive, got)
}

func TestPullArchiveIgnoresAProvenanceLayer(t *testing.T) {
	archive := validArchive(t)
	f := newFakeRegistry()
	config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{}`))
	chartLayer := f.addBlob(helmregistry.ChartLayerMediaType, archive)
	prov := f.addBlob(helmregistry.ProvLayerMediaType, []byte("signature"))
	f.addManifest(t, ocispec.Manifest{Config: config, Layers: []ocispec.Descriptor{chartLayer, prov}})
	pinned := f.pin(t)

	got, err := PullArchive(context.Background(), serveRepo(t, f), pinned, FetchOptions{PlainHTTP: true})

	require.NoError(t, err)
	require.Equal(t, archive, got)
	require.False(t, f.wasRequested(blobPath(prov)), "the provenance layer is not downloaded")
}

func TestPullArchiveDoesNotOpenTheArchive(t *testing.T) {
	f := newFakeRegistry()
	f.addChart(t, []byte("this is not an archive"))
	pinned := f.pin(t)

	got, err := PullArchive(context.Background(), serveRepo(t, f), pinned, FetchOptions{PlainHTTP: true})

	require.NoError(t, err)
	require.Equal(t, []byte("this is not an archive"), got)
}

func TestPullArchiveFails(t *testing.T) {
	otherDigest := digest.FromString("another manifest").String()

	tests := []struct {
		name    string
		setup   func(t *testing.T, f *fakeRegistry) (pinned string)
		wantErr string
		// notRequested is a blob path that must not have been downloaded.
		notRequested func() string
	}{
		{
			name: "no manifest has the digest",
			setup: func(t *testing.T, f *fakeRegistry) string {
				f.addChart(t, validArchive(t))
				return otherDigest
			},
			wantErr: "was not found in the registry",
		},
		{
			name: "the registry does not allow reading without credentials",
			setup: func(t *testing.T, f *fakeRegistry) string {
				f.addChart(t, validArchive(t))
				pinned := f.pin(t)
				f.status["/v2/"+testRepository+"/manifests/"+pinned] = http.StatusUnauthorized
				return pinned
			},
			wantErr: "does not allow reading",
		},
		{
			name: "the registry answers with another manifest than the pinned one",
			setup: func(t *testing.T, f *fakeRegistry) string {
				f.addChart(t, validArchive(t))
				f.pin(t)
				// What is asked for is otherDigest, but the registry sends the chart.
				f.responses["/v2/"+testRepository+"/manifests/"+otherDigest] = f.responses["/v2/"+testRepository+"/manifests/"+testTag]
				return otherDigest
			},
			wantErr: "digest mismatch in Docker-Content-Digest",
		},
		{
			name: "the artifact is not a Helm chart",
			setup: func(t *testing.T, f *fakeRegistry) string {
				config := f.addBlob(ocispec.MediaTypeImageConfig, []byte(`{}`))
				layer := f.addBlob(ocispec.MediaTypeImageLayerGzip, validArchive(t))
				f.addManifest(t, ocispec.Manifest{Config: config, Layers: []ocispec.Descriptor{layer}})
				return f.pin(t)
			},
			wantErr: "is not a Helm chart",
		},
		{
			name: "no chart layer",
			setup: func(t *testing.T, f *fakeRegistry) string {
				config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{}`))
				f.addManifest(t, ocispec.Manifest{Config: config})
				return f.pin(t)
			},
			wantErr: "it has no chart layer",
		},
		{
			name: "a chart layer larger than the limit",
			setup: func(t *testing.T, f *fakeRegistry) string {
				config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{}`))
				huge := ocispec.Descriptor{
					MediaType: helmregistry.ChartLayerMediaType,
					Digest:    digest.FromString("huge"),
					Size:      maxLayerSize + 1,
				}
				f.addManifest(t, ocispec.Manifest{Config: config, Layers: []ocispec.Descriptor{huge}})
				return f.pin(t)
			},
			wantErr: "larger than the 2097152 bytes that are read",
			notRequested: func() string {
				return "/v2/" + testRepository + "/blobs/" + digest.FromString("huge").String()
			},
		},
		{
			name: "the registry sends other bytes for the chart layer",
			setup: func(t *testing.T, f *fakeRegistry) string {
				layer := f.addChart(t, validArchive(t))
				tampered := bytes.Repeat([]byte("x"), int(layer.Size))
				f.responses[blobPath(layer)] = response{contentType: helmregistry.ChartLayerMediaType, body: tampered}
				return f.pin(t)
			},
			wantErr: "digest mismatch",
		},
		{
			name: "the chart layer is missing",
			setup: func(t *testing.T, f *fakeRegistry) string {
				layer := f.addChart(t, validArchive(t))
				delete(f.responses, blobPath(layer))
				return f.pin(t)
			},
			wantErr: "was not found in the registry",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRegistry()
			pinned := tt.setup(t, f)

			got, err := PullArchive(context.Background(), serveRepo(t, f), pinned, FetchOptions{PlainHTTP: true})

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
			if tt.notRequested != nil {
				require.False(t, f.wasRequested(tt.notRequested()), "the layer must not be downloaded")
			}
		})
	}
}

func TestPullArchiveRefusesADigestThatIsNotSha256(t *testing.T) {
	f := newFakeRegistry()
	f.addChart(t, validArchive(t))
	repo := serveRepo(t, f)

	tests := []struct {
		name   string
		digest string
	}{
		{
			name:   "empty",
			digest: "",
		},
		{
			name:   "a tag",
			digest: testTag,
		},
		{
			name:   "too short",
			digest: "sha256:0123",
		},
		{
			name:   "another algorithm",
			digest: "sha512:" + strings.Repeat("0", 128),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PullArchive(context.Background(), repo, tt.digest, FetchOptions{PlainHTTP: true})

			require.ErrorContains(t, err, "is not a sha256 digest")
			require.Nil(t, got)
			require.Empty(t, f.requested, "nothing is asked of the registry")
		})
	}
}
