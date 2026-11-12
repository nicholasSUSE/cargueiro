package index

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	repo "helm.sh/helm/v4/pkg/repo/v1"

	"github.com/rancher/cargueiro/internal/oci"
)

// pullCall is one call to the fake pull function.
type pullCall struct {
	repo      oci.ChartRepo
	digest    string
	plainHTTP bool
}

// fakePull answers with the archives it is given, by digest, and records its calls.
type fakePull struct {
	archives map[string][]byte
	failures map[string]error
	calls    []pullCall
}

func (f *fakePull) pull(_ context.Context, r oci.ChartRepo, digest string, opts oci.FetchOptions) ([]byte, error) {
	f.calls = append(f.calls, pullCall{repo: r, digest: digest, plainHTTP: opts.PlainHTTP})
	if err := f.failures[digest]; err != nil {
		return nil, err
	}
	return f.archives[digest], nil
}

// entryPinnedTo returns the fleet entry of a version, pinned to the digest.
func entryPinnedTo(version, digest string) *repo.ChartVersion {
	e := fleetEntry()
	e.Version = version
	e.URLs = []string{"oci://registry.example.com/rancher/fleet:" + strings.ReplaceAll(version, "+", "_") + "@" + digest}
	return e
}

func TestPullAdded(t *testing.T) {
	const digestOne = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	archive := archiveOf(t, fleetChartYAML)

	t.Run("each added version, from the manifest its url pins", func(t *testing.T) {
		fake := &fakePull{archives: map[string][]byte{digestOne: archive}}
		all := indexOf("fleet", entryPinnedTo("111.0.3+up0.17.3", digestOne))
		added := []Ref{{Chart: "fleet", Version: "111.0.3+up0.17.3"}}

		got, err := pullAdded(context.Background(), all, added, Options{PlainHTTP: true, Pull: fake.pull})

		require.NoError(t, err)
		require.Equal(t, map[Ref][]byte{{Chart: "fleet", Version: "111.0.3+up0.17.3"}: archive}, got)
		require.Equal(t, []pullCall{{
			repo:      oci.ChartRepo{Host: "registry.example.com", Repository: "rancher/fleet"},
			digest:    digestOne,
			plainHTTP: true,
		}}, fake.calls)
	})

	t.Run("nothing added, nothing pulled", func(t *testing.T) {
		fake := &fakePull{}

		got, err := pullAdded(context.Background(), indexOf("fleet", fleetEntry()), nil, Options{Pull: fake.pull})

		require.NoError(t, err)
		require.Empty(t, got)
		require.Empty(t, fake.calls)
	})
}

func TestPullAddedFails(t *testing.T) {
	const digest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	added := []Ref{{Chart: "fleet", Version: "111.0.3+up0.17.3"}}

	tests := []struct {
		name    string
		fake    *fakePull
		wantErr string
	}{
		{
			name:    "the registry fails",
			fake:    &fakePull{failures: map[string]error{digest: errors.New("the registry is down")}},
			wantErr: "fleet 111.0.3+up0.17.3: pull the archive: the registry is down",
		},
		{
			name:    "the archive is not the chart of the entry",
			fake:    &fakePull{archives: map[string][]byte{digest: archiveOf(t, "apiVersion: v2\nname: fleet\nversion: 9.9.9\n")}},
			wantErr: "fleet 111.0.3+up0.17.3: the Chart.yaml in the archive differs from the entry in:",
		},
		{
			name:    "what is pulled is not an archive",
			fake:    &fakePull{archives: map[string][]byte{digest: []byte("not an archive")}},
			wantErr: "fleet 111.0.3+up0.17.3: read the archive:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			all := indexOf("fleet", entryPinnedTo("111.0.3+up0.17.3", digest))

			got, err := pullAdded(context.Background(), all, added, Options{Pull: tt.fake.pull})

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestPullAddedNamesEveryVersionAtFault(t *testing.T) {
	const (
		digestOne = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		digestTwo = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	)
	fake := &fakePull{failures: map[string]error{
		digestOne: errors.New("down"),
		digestTwo: errors.New("down"),
	}}
	all := indexOf("fleet", entryPinnedTo("1", digestOne), entryPinnedTo("2", digestTwo))
	added := []Ref{{Chart: "fleet", Version: "1"}, {Chart: "fleet", Version: "2"}}

	_, err := pullAdded(context.Background(), all, added, Options{Pull: fake.pull})

	require.ErrorContains(t, err, "fleet 1: pull the archive: down")
	require.ErrorContains(t, err, "fleet 2: pull the archive: down")
	require.Len(t, fake.calls, 2, "a failure does not stop the others from being tried")
}

func TestPullAddedUsesTheRegistryByDefault(t *testing.T) {
	// Without a Pull function, oci.PullArchive is used: a registry that does not
	// exist fails, and is named.
	all := indexOf("fleet", entryPinnedTo("111.0.3+up0.17.3", "sha256:1111111111111111111111111111111111111111111111111111111111111111"))
	all.Entries["fleet"][0].URLs = []string{"oci://127.0.0.1:1/rancher/fleet:111.0.3_up0.17.3@sha256:1111111111111111111111111111111111111111111111111111111111111111"}

	got, err := pullAdded(context.Background(), all, []Ref{{Chart: "fleet", Version: "111.0.3+up0.17.3"}}, Options{PlainHTTP: true})

	require.ErrorContains(t, err, "fleet 111.0.3+up0.17.3: pull the archive:")
	require.Nil(t, got)
}
