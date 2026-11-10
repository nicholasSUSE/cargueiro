package chart

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// entry is one item of a test archive.
type entry struct {
	name     string
	body     string
	typeflag byte
	linkname string
}

// buildArchive writes a .tgz holding entries, in order. A zero typeflag is a
// regular file.
func buildArchive(t *testing.T, entries []entry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typeflag := e.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: typeflag,
			Linkname: e.linkname,
			Mode:     0o644,
		}
		if typeflag == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestLoadArchiveOfARealChart(t *testing.T) {
	data, err := os.ReadFile("../../tests/data/charts/fleet/published/fleet-110.0.3+up0.16.3.tgz")
	require.NoError(t, err)

	got, err := LoadArchive(bytes.NewReader(data))
	require.NoError(t, err)

	require.Equal(t, "fleet", got.Metadata.Name)
	require.Equal(t, "110.0.3+up0.16.3", got.Metadata.Version)
	require.Equal(t, "rancher", got.Annotations["catalog.cattle.io/certified"])
	require.Contains(t, got.Annotations, "helm.sh/images")
	require.NotEmpty(t, got.Values)
}

func TestLoadArchive(t *testing.T) {
	tests := []struct {
		name       string
		entries    []entry
		wantName   string
		wantValues map[string]any
	}{
		{
			name: "the files in the order Helm writes them",
			entries: []entry{
				{name: "fleet/Chart.yaml", body: "name: fleet\n"},
				{name: "fleet/values.yaml", body: "a: 1\n"},
				{name: "fleet/templates/x.yaml", body: "b: 2\n"},
			},
			wantName:   "fleet",
			wantValues: map[string]any{"a": 1},
		},
		{
			name: "no values.yaml, as in most CRD charts",
			entries: []entry{
				{name: "fleet-crd/Chart.yaml", body: "name: fleet-crd\n"},
				{name: "fleet-crd/templates/crds.yaml", body: "b: 2\n"},
			},
			wantName:   "fleet-crd",
			wantValues: nil,
		},
		{
			name: "values.yaml before Chart.yaml",
			entries: []entry{
				{name: "fleet/values.yaml", body: "a: 1\n"},
				{name: "fleet/Chart.yaml", body: "name: fleet\n"},
			},
			wantName:   "fleet",
			wantValues: map[string]any{"a": 1},
		},
		{
			name: "a leading ./ is ignored",
			entries: []entry{
				{name: "./fleet/Chart.yaml", body: "name: fleet\n"},
				{name: "./fleet/values.yaml", body: "a: 1\n"},
			},
			wantName:   "fleet",
			wantValues: map[string]any{"a": 1},
		},
		{
			name: "the same names in a subchart are ignored",
			entries: []entry{
				{name: "fleet/charts/sub/Chart.yaml", body: "name: sub\n"},
				{name: "fleet/charts/sub/values.yaml", body: "sub: true\n"},
				{name: "fleet/Chart.yaml", body: "name: fleet\n"},
				{name: "fleet/values.yaml", body: "a: 1\n"},
			},
			wantName:   "fleet",
			wantValues: map[string]any{"a": 1},
		},
		{
			name: "a link with the name of a file is skipped",
			entries: []entry{
				{name: "fleet/Chart.yaml", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"},
				{name: "fleet/Chart.yaml", body: "name: fleet\n"},
				{name: "fleet/values.yaml", body: "a: 1\n"},
			},
			wantName:   "fleet",
			wantValues: map[string]any{"a": 1},
		},
		{
			name: "an empty values.yaml",
			entries: []entry{
				{name: "fleet/Chart.yaml", body: "name: fleet\n"},
				{name: "fleet/values.yaml", body: "# nothing\n"},
			},
			wantName:   "fleet",
			wantValues: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadArchive(bytes.NewReader(buildArchive(t, tt.entries)))
			require.NoError(t, err)
			require.Equal(t, tt.wantName, got.Metadata.Name)
			require.Equal(t, tt.wantValues, got.Values)
		})
	}
}

func TestLoadArchiveRejects(t *testing.T) {
	small := limits{
		file:   64,
		stream: 4096,
	}

	tests := []struct {
		name    string
		data    func(t *testing.T) []byte
		lim     limits
		wantErr string
	}{
		{
			name: "not gzip",
			data: func(*testing.T) []byte {
				return []byte("this is not an archive")
			},
			lim:     defaultLimits(),
			wantErr: "the archive is not gzip",
		},
		{
			name: "gzip, but not tar",
			data: func(t *testing.T) []byte {
				var buf bytes.Buffer
				gz := gzip.NewWriter(&buf)
				_, err := gz.Write([]byte(strings.Repeat("not a tar file ", 100)))
				require.NoError(t, err)
				require.NoError(t, gz.Close())
				return buf.Bytes()
			},
			lim:     defaultLimits(),
			wantErr: "read the archive",
		},
		{
			name: "no Chart.yaml",
			data: func(t *testing.T) []byte {
				return buildArchive(t, []entry{{name: "fleet/values.yaml", body: "a: 1\n"}})
			},
			lim:     defaultLimits(),
			wantErr: "the archive has no Chart.yaml",
		},
		{
			name: "the files are not inside a chart directory",
			data: func(t *testing.T) []byte {
				return buildArchive(t, []entry{
					{name: "Chart.yaml", body: "name: fleet\n"},
					{name: "values.yaml", body: "a: 1\n"},
				})
			},
			lim:     defaultLimits(),
			wantErr: "the archive has no Chart.yaml",
		},
		{
			name: "Chart.yaml twice",
			data: func(t *testing.T) []byte {
				return buildArchive(t, []entry{
					{name: "fleet/Chart.yaml", body: "name: fleet\n"},
					{name: "fleet/Chart.yaml", body: "name: other\n"},
				})
			},
			lim:     defaultLimits(),
			wantErr: "fleet/Chart.yaml appears twice in the archive",
		},
		{
			name: "a file over the limit",
			data: func(t *testing.T) []byte {
				return buildArchive(t, []entry{
					{name: "fleet/Chart.yaml", body: "name: fleet\n"},
					{name: "fleet/values.yaml", body: "a: " + strings.Repeat("x", 100) + "\n"},
				})
			},
			lim:     small,
			wantErr: "fleet/values.yaml is larger than 64 bytes",
		},
		{
			name: "an archive that is too large once decompressed",
			data: func(t *testing.T) []byte {
				return buildArchive(t, []entry{
					{name: "fleet/padding", body: strings.Repeat("0", 10000)},
					{name: "fleet/Chart.yaml", body: "name: fleet\n"},
					{name: "fleet/values.yaml", body: "a: 1\n"},
				})
			},
			lim:     small,
			wantErr: "too large once decompressed",
		},
		{
			name: "Chart.yaml is not yaml",
			data: func(t *testing.T) []byte {
				return buildArchive(t, []entry{
					{name: "fleet/Chart.yaml", body: "name: [\n"},
					{name: "fleet/values.yaml", body: "a: 1\n"},
				})
			},
			lim:     defaultLimits(),
			wantErr: "parse Chart.yaml",
		},
		{
			name: "values.yaml is a list",
			data: func(t *testing.T) []byte {
				return buildArchive(t, []entry{
					{name: "fleet/Chart.yaml", body: "name: fleet\n"},
					{name: "fleet/values.yaml", body: "- a\n- b\n"},
				})
			},
			lim:     defaultLimits(),
			wantErr: "parse values.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadArchive(bytes.NewReader(tt.data(t)), tt.lim)
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}
