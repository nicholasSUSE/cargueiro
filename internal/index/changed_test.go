package index

import (
	"testing"

	"github.com/stretchr/testify/require"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// entry returns the version of a chart as an index lists it.
func entry(name, version, description string) *repo.ChartVersion {
	return &repo.ChartVersion{
		Metadata: &chart.Metadata{
			Name:        name,
			Version:     version,
			Description: description,
		},
		URLs: []string{"oci://registry.example.com/" + name + ":" + version},
	}
}

// indexOf returns an index of the given versions, which are all of one chart.
func indexOf(name string, versions ...*repo.ChartVersion) *repo.IndexFile {
	return &repo.IndexFile{
		APIVersion: "v1",
		Entries:    map[string]repo.ChartVersions{name: versions},
	}
}

func TestChangedRefs(t *testing.T) {
	changedURL := entry("fleet", "2", "d")
	changedURL.URLs = []string{"oci://other.example.com/fleet:2"}
	withDigest := entry("fleet", "2", "d")
	withDigest.Digest = "abc"

	tests := []struct {
		name        string
		published   *repo.IndexFile
		regenerated *repo.IndexFile
		want        []Ref
	}{
		{
			name:        "nothing is published yet",
			published:   nil,
			regenerated: indexOf("fleet", entry("fleet", "1", "d")),
			want:        nil,
		},
		{
			name:        "the same entries",
			published:   indexOf("fleet", entry("fleet", "1", "d"), entry("fleet", "2", "d")),
			regenerated: indexOf("fleet", entry("fleet", "2", "d"), entry("fleet", "1", "d")),
			want:        nil,
		},
		{
			name:        "a new entry is not a change",
			published:   indexOf("fleet", entry("fleet", "1", "d")),
			regenerated: indexOf("fleet", entry("fleet", "1", "d"), entry("fleet", "2", "d")),
			want:        nil,
		},
		{
			name:        "a version that is gone is left to State.Plan",
			published:   indexOf("fleet", entry("fleet", "1", "d"), entry("fleet", "2", "d")),
			regenerated: indexOf("fleet", entry("fleet", "1", "d")),
			want:        nil,
		},
		{
			name:        "a description that changed",
			published:   indexOf("fleet", entry("fleet", "1", "d")),
			regenerated: indexOf("fleet", entry("fleet", "1", "another description")),
			want:        []Ref{{Chart: "fleet", Version: "1"}},
		},
		{
			name:        "a URL that changed",
			published:   indexOf("fleet", entry("fleet", "2", "d")),
			regenerated: indexOf("fleet", changedURL),
			want:        []Ref{{Chart: "fleet", Version: "2"}},
		},
		{
			name:        "a digest that was added",
			published:   indexOf("fleet", entry("fleet", "2", "d")),
			regenerated: indexOf("fleet", withDigest),
			want:        []Ref{{Chart: "fleet", Version: "2"}},
		},
		{
			name: "every change is reported, sorted",
			published: &repo.IndexFile{
				APIVersion: "v1",
				Entries: map[string]repo.ChartVersions{
					"fleet":    {entry("fleet", "2", "d"), entry("fleet", "1", "d")},
					"alerting": {entry("alerting", "9", "d")},
				},
			},
			regenerated: &repo.IndexFile{
				APIVersion: "v1",
				Entries: map[string]repo.ChartVersions{
					"fleet":    {entry("fleet", "2", "x"), entry("fleet", "1", "x")},
					"alerting": {entry("alerting", "9", "x")},
				},
			},
			want: []Ref{
				{Chart: "alerting", Version: "9"},
				{Chart: "fleet", Version: "1"},
				{Chart: "fleet", Version: "2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := changedRefs(tt.published, tt.regenerated)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCheckUnchanged(t *testing.T) {
	t.Run("nothing changed", func(t *testing.T) {
		published := indexOf("fleet", entry("fleet", "1", "d"))

		require.NoError(t, checkUnchanged(published, indexOf("fleet", entry("fleet", "1", "d"))))
	})

	t.Run("nothing is published yet", func(t *testing.T) {
		require.NoError(t, checkUnchanged(nil, indexOf("fleet", entry("fleet", "1", "d"))))
	})

	t.Run("every version that changed is named", func(t *testing.T) {
		published := indexOf("fleet", entry("fleet", "1", "d"), entry("fleet", "2", "d"))
		regenerated := indexOf("fleet", entry("fleet", "1", "x"), entry("fleet", "2", "x"))

		err := checkUnchanged(published, regenerated)

		require.ErrorContains(t, err, "fleet 1 is in index.yaml and its entry no longer matches it: a published version never changes")
		require.ErrorContains(t, err, "fleet 2 is in index.yaml and its entry no longer matches it: a published version never changes")
	})
}
