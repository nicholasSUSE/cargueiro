package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	helmregistry "helm.sh/helm/v4/pkg/registry"
)

const (
	testRepository = "rancher/fleet"
	testTag        = "1.0.0"
)

// response is what the fake registry answers for one path.
type response struct {
	contentType string
	body        []byte
	// digest is sent as the digest of the body when it is set. It lets a test
	// make the registry lie about the content.
	digest string
}

// fakeRegistry serves manifests and blobs from memory, and records the paths
// it was asked for.
type fakeRegistry struct {
	mu        sync.Mutex
	responses map[string]response
	status    map[string]int
	requested []string
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		responses: map[string]response{},
		status:    map[string]int{},
	}
}

func (f *fakeRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requested = append(f.requested, r.URL.Path)
	forced, isForced := f.status[r.URL.Path]
	resp, found := f.responses[r.URL.Path]
	f.mu.Unlock()

	switch {
	case isForced:
		w.WriteHeader(forced)
	case !found:
		w.WriteHeader(http.StatusNotFound)
	default:
		w.Header().Set("Content-Type", resp.contentType)
		sent := resp.digest
		if sent == "" {
			sent = digest.FromBytes(resp.body).String()
		}
		w.Header().Set("Docker-Content-Digest", sent)
		w.Header().Set("Content-Length", strconv.Itoa(len(resp.body)))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(resp.body)
	}
}

func (f *fakeRegistry) wasRequested(path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.requested {
		if p == path {
			return true
		}
	}
	return false
}

// addBlob stores body and returns its descriptor.
func (f *fakeRegistry) addBlob(mediaType string, body []byte) ocispec.Descriptor {
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(body),
		Size:      int64(len(body)),
	}
	f.responses[blobPath(desc)] = response{contentType: mediaType, body: body}
	return desc
}

// addManifest stores a manifest under testTag.
func (f *fakeRegistry) addManifest(t *testing.T, manifest ocispec.Manifest) {
	t.Helper()
	manifest.SchemaVersion = 2
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	f.responses["/v2/"+testRepository+"/manifests/"+testTag] = response{
		contentType: ocispec.MediaTypeImageManifest,
		body:        body,
	}
}

func blobPath(desc ocispec.Descriptor) string {
	return "/v2/" + testRepository + "/blobs/" + desc.Digest.String()
}

// addChart stores a Helm chart with archive as its chart layer, and returns the
// descriptor of that layer.
func (f *fakeRegistry) addChart(t *testing.T, archive []byte) ocispec.Descriptor {
	t.Helper()
	config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{"name":"fleet","version":"1.0.0"}`))
	layer := f.addBlob(helmregistry.ChartLayerMediaType, archive)
	f.addManifest(t, ocispec.Manifest{
		Config: config,
		Layers: []ocispec.Descriptor{layer},
	})
	return layer
}

// tgz builds a chart archive from name/content pairs.
func tgz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(body)),
		}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func validArchive(t *testing.T) []byte {
	return tgz(t, map[string]string{
		"fleet/Chart.yaml":  "name: fleet\nversion: 1.0.0\nannotations:\n  catalog.cattle.io/certified: rancher\n",
		"fleet/values.yaml": "image:\n  repository: rancher/fleet\n  tag: v1\n",
	})
}

// serve starts f and returns the reference to the test chart on it.
func serve(t *testing.T, f *fakeRegistry) ChartRef {
	t.Helper()
	srv := newServer(f)
	t.Cleanup(srv.Close)
	return ChartRef{
		Host:       strings.TrimPrefix(srv.URL, "http://"),
		Repository: testRepository,
		Tag:        testTag,
	}
}

// newServer starts a server for f that does not log failed connections, such
// as the TLS handshakes of a client that expects HTTPS.
func newServer(f *fakeRegistry) *httptest.Server {
	srv := httptest.NewUnstartedServer(f)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	return srv
}

func TestFetchChart(t *testing.T) {
	f := newFakeRegistry()
	f.addChart(t, validArchive(t))

	got, err := FetchChart(context.Background(), serve(t, f), FetchOptions{PlainHTTP: true})

	require.NoError(t, err)
	require.Equal(t, "fleet", got.Metadata.Name)
	require.Equal(t, "1.0.0", got.Metadata.Version)
	require.Equal(t, "rancher", got.Annotations["catalog.cattle.io/certified"])
	require.Equal(t, map[string]any{
		"image": map[string]any{"repository": "rancher/fleet", "tag": "v1"},
	}, got.Values)
}

func TestFetchChartWithoutValues(t *testing.T) {
	f := newFakeRegistry()
	f.addChart(t, tgz(t, map[string]string{"fleet-crd/Chart.yaml": "name: fleet-crd\n"}))

	got, err := FetchChart(context.Background(), serve(t, f), FetchOptions{PlainHTTP: true})

	require.NoError(t, err)
	require.Equal(t, "fleet-crd", got.Metadata.Name)
	require.Empty(t, got.Values)
}

func TestFetchChartIgnoresAProvenanceLayer(t *testing.T) {
	f := newFakeRegistry()
	config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{}`))
	chartLayer := f.addBlob(helmregistry.ChartLayerMediaType, validArchive(t))
	prov := f.addBlob(helmregistry.ProvLayerMediaType, []byte("signature"))
	f.addManifest(t, ocispec.Manifest{
		Config: config,
		Layers: []ocispec.Descriptor{chartLayer, prov},
	})

	got, err := FetchChart(context.Background(), serve(t, f), FetchOptions{PlainHTTP: true})

	require.NoError(t, err)
	require.Equal(t, "fleet", got.Metadata.Name)
	require.False(t, f.wasRequested(blobPath(prov)), "the provenance layer is not downloaded")
}

func TestFetchChartFails(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, f *fakeRegistry)
		wantErr string
		// notRequested is a blob path that must not have been downloaded.
		notRequested func(f *fakeRegistry) string
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
		{
			name: "the artifact is not a Helm chart",
			setup: func(t *testing.T, f *fakeRegistry) {
				config := f.addBlob(ocispec.MediaTypeImageConfig, []byte(`{}`))
				layer := f.addBlob(ocispec.MediaTypeImageLayerGzip, validArchive(t))
				f.addManifest(t, ocispec.Manifest{Config: config, Layers: []ocispec.Descriptor{layer}})
			},
			wantErr: "is not a Helm chart",
		},
		{
			name: "the manifest is an index",
			setup: func(_ *testing.T, f *fakeRegistry) {
				f.responses["/v2/"+testRepository+"/manifests/"+testTag] = response{
					contentType: ocispec.MediaTypeImageIndex,
					body:        []byte(`{"schemaVersion":2,"manifests":[]}`),
				}
			},
			wantErr: "is not a Helm chart",
		},
		{
			name: "the manifest is not json",
			setup: func(_ *testing.T, f *fakeRegistry) {
				f.responses["/v2/"+testRepository+"/manifests/"+testTag] = response{
					contentType: ocispec.MediaTypeImageManifest,
					body:        []byte("not json"),
				}
			},
			wantErr: "is not a Helm chart",
		},
		{
			name: "no chart layer",
			setup: func(t *testing.T, f *fakeRegistry) {
				config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{}`))
				f.addManifest(t, ocispec.Manifest{Config: config})
			},
			wantErr: "it has no chart layer",
		},
		{
			name: "two chart layers",
			setup: func(t *testing.T, f *fakeRegistry) {
				config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{}`))
				first := f.addBlob(helmregistry.ChartLayerMediaType, validArchive(t))
				second := f.addBlob(helmregistry.ChartLayerMediaType, []byte("another"))
				f.addManifest(t, ocispec.Manifest{Config: config, Layers: []ocispec.Descriptor{first, second}})
			},
			wantErr: "more than one chart layer",
		},
		{
			name: "a chart layer larger than the limit",
			setup: func(t *testing.T, f *fakeRegistry) {
				config := f.addBlob(helmregistry.ConfigMediaType, []byte(`{}`))
				huge := ocispec.Descriptor{
					MediaType: helmregistry.ChartLayerMediaType,
					Digest:    digest.FromString("huge"),
					Size:      maxLayerSize + 1,
				}
				f.addManifest(t, ocispec.Manifest{Config: config, Layers: []ocispec.Descriptor{huge}})
			},
			wantErr: "larger than the 2097152 bytes that are read",
			notRequested: func(*fakeRegistry) string {
				return "/v2/" + testRepository + "/blobs/" + digest.FromString("huge").String()
			},
		},
		{
			name: "the registry sends other bytes, and says so",
			setup: func(t *testing.T, f *fakeRegistry) {
				layer := f.addChart(t, validArchive(t))
				tampered := bytes.Repeat([]byte("x"), int(layer.Size))
				f.responses[blobPath(layer)] = response{contentType: helmregistry.ChartLayerMediaType, body: tampered}
			},
			wantErr: "digest mismatch",
		},
		{
			name: "the registry sends other bytes and claims they are the right ones",
			setup: func(t *testing.T, f *fakeRegistry) {
				layer := f.addChart(t, validArchive(t))
				tampered := bytes.Repeat([]byte("x"), int(layer.Size))
				f.responses[blobPath(layer)] = response{
					contentType: helmregistry.ChartLayerMediaType,
					body:        tampered,
					digest:      layer.Digest.String(),
				}
			},
			wantErr: "read the chart layer",
		},
		{
			name: "the chart layer is missing",
			setup: func(t *testing.T, f *fakeRegistry) {
				layer := f.addChart(t, validArchive(t))
				delete(f.responses, blobPath(layer))
			},
			wantErr: "was not found in the registry",
		},
		{
			name: "the chart layer is not gzip",
			setup: func(t *testing.T, f *fakeRegistry) {
				f.addChart(t, []byte("this is not an archive"))
			},
			wantErr: "the archive is not gzip",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRegistry()
			tt.setup(t, f)

			got, err := FetchChart(context.Background(), serve(t, f), FetchOptions{PlainHTTP: true})

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
			if tt.notRequested != nil {
				require.False(t, f.wasRequested(tt.notRequested(f)), "the layer must not be downloaded")
			}
		})
	}
}

func TestFetchChartCannotReachTheRegistry(t *testing.T) {
	f := newFakeRegistry()
	f.addChart(t, validArchive(t))
	srv := newServer(f)
	ref := ChartRef{
		Host:       strings.TrimPrefix(srv.URL, "http://"),
		Repository: testRepository,
		Tag:        testTag,
	}

	t.Run("it is down", func(t *testing.T) {
		srv.Close()
		got, err := FetchChart(context.Background(), ref, FetchOptions{PlainHTTP: true})
		require.ErrorContains(t, err, "fetch "+ref.String())
		require.Nil(t, got)
	})
}

func TestFetchChartOverHTTPSNeedsHTTPS(t *testing.T) {
	f := newFakeRegistry()
	f.addChart(t, validArchive(t))

	got, err := FetchChart(context.Background(), serve(t, f), FetchOptions{})

	require.ErrorContains(t, err, "fetch ")
	require.Nil(t, got)
}
