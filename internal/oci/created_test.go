package oci

import (
	"context"
	"testing"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	helmregistry "helm.sh/helm/v4/pkg/registry"
)

// addChartCreated stores a Helm chart whose manifest has the annotations given.
func (f *fakeRegistry) addChartCreated(t *testing.T, annotations map[string]string) {
	t.Helper()

	config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{"name":"fleet","version":"1.0.0"}`))
	layer := f.addBlob(helmregistry.ChartLayerMediaType, validArchive(t))
	f.addManifest(t, ocispec.Manifest{Config: config, Layers: []ocispec.Descriptor{layer}, Annotations: annotations})
}

func TestManifestCreated(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        time.Time
		wantErr     string
	}{
		{
			name:        "the time Helm recorded",
			annotations: map[string]string{ocispec.AnnotationCreated: "2026-11-11T17:33:45Z"},
			want:        time.Date(2026, 11, 11, 17, 33, 45, 0, time.UTC),
		},
		{
			name:        "a time with another offset is given in UTC",
			annotations: map[string]string{ocispec.AnnotationCreated: "2026-11-11T14:33:45-03:00"},
			want:        time.Date(2026, 11, 11, 17, 33, 45, 0, time.UTC),
		},
		{
			name:        "other annotations are ignored",
			annotations: map[string]string{ocispec.AnnotationCreated: "2026-11-11T17:33:45Z", ocispec.AnnotationTitle: "fleet", "catalog.cattle.io/certified": "rancher"},
			want:        time.Date(2026, 11, 11, 17, 33, 45, 0, time.UTC),
		},
		{
			name:        "no annotation",
			annotations: map[string]string{ocispec.AnnotationTitle: "fleet"},
			wantErr:     "has no org.opencontainers.image.created annotation, which says when the chart was pushed",
		},
		{
			name:    "no annotations at all",
			wantErr: "has no org.opencontainers.image.created annotation",
		},
		{
			name:        "an annotation that is not a time",
			annotations: map[string]string{ocispec.AnnotationCreated: "yesterday"},
			wantErr:     `is "yesterday", which is not a time like 2006-01-02T15:04:05Z`,
		},
		{
			name:        "an empty annotation",
			annotations: map[string]string{ocispec.AnnotationCreated: ""},
			wantErr:     `is "", which is not a time`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRegistry()
			f.addChartCreated(t, tt.annotations)
			pinned := f.pin(t)

			got, err := ManifestCreated(context.Background(), serveRepo(t, f), pinned, FetchOptions{PlainHTTP: true})

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.True(t, got.IsZero())
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestManifestCreatedFails(t *testing.T) {
	t.Run("a digest that is not sha256", func(t *testing.T) {
		_, err := ManifestCreated(context.Background(), ChartRepo{Host: "registry.example.com", Repository: "fleet"}, "latest", FetchOptions{})

		require.ErrorContains(t, err, `"latest" is not a sha256 digest`)
	})

	t.Run("a manifest that is not there", func(t *testing.T) {
		f := newFakeRegistry()
		f.addChartCreated(t, map[string]string{ocispec.AnnotationCreated: "2026-11-11T17:33:45Z"})
		f.pin(t)

		_, err := ManifestCreated(context.Background(), serveRepo(t, f),
			"sha256:0000000000000000000000000000000000000000000000000000000000000000", FetchOptions{PlainHTTP: true})

		require.Error(t, err)
	})
}
