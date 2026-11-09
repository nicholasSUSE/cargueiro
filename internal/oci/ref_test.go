package oci

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseChartRef(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want ChartRef
	}{
		{
			name: "host, one path segment and a tag",
			in:   "registry.example.com/fleet:1.0.0",
			want: ChartRef{Host: "registry.example.com", Repository: "fleet", Tag: "1.0.0"},
		},
		{
			name: "a nested repository",
			in:   "registry.example.com/rancher/charts/fleet:1.0.0",
			want: ChartRef{Host: "registry.example.com", Repository: "rancher/charts/fleet", Tag: "1.0.0"},
		},
		{
			name: "a port",
			in:   "127.0.0.1:5050/rancher/fleet:1.0.0",
			want: ChartRef{Host: "127.0.0.1:5050", Repository: "rancher/fleet", Tag: "1.0.0"},
		},
		{
			name: "the oci:// scheme",
			in:   "oci://registry.example.com/rancher/fleet:1.0.0",
			want: ChartRef{Host: "registry.example.com", Repository: "rancher/fleet", Tag: "1.0.0"},
		},
		{
			name: "a plus in the version is an underscore in the tag",
			in:   "registry.example.com/rancher/fleet:110.0.3+up0.16.3",
			want: ChartRef{Host: "registry.example.com", Repository: "rancher/fleet", Tag: "110.0.3_up0.16.3"},
		},
		{
			name: "the tag as the registry stores it",
			in:   "registry.example.com/rancher/fleet:110.0.3_up0.16.3",
			want: ChartRef{Host: "registry.example.com", Repository: "rancher/fleet", Tag: "110.0.3_up0.16.3"},
		},
		{
			name: "a release candidate",
			in:   "registry.example.com/rancher/fleet:1.0.0-rc.1",
			want: ChartRef{Host: "registry.example.com", Repository: "rancher/fleet", Tag: "1.0.0-rc.1"},
		},
		{
			name: "space around the reference",
			in:   "  registry.example.com/fleet:1.0.0 \n",
			want: ChartRef{Host: "registry.example.com", Repository: "fleet", Tag: "1.0.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChartRef(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestChartRefString(t *testing.T) {
	ref := ChartRef{Host: "127.0.0.1:5050", Repository: "rancher/fleet", Tag: "110.0.3_up0.16.3"}
	require.Equal(t, "127.0.0.1:5050/rancher/fleet:110.0.3_up0.16.3", ref.String())
}

func TestParseChartRefRejects(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{
			name:    "empty",
			in:      "",
			wantErr: "empty reference",
		},
		{
			name:    "only a host",
			in:      "registry.example.com",
			wantErr: "want host/path/chart:tag",
		},
		{
			name:    "a host and a port, no repository",
			in:      "127.0.0.1:5050",
			wantErr: "want host/path/chart:tag",
		},
		{
			name:    "no tag",
			in:      "registry.example.com/rancher/fleet",
			wantErr: "has no tag",
		},
		{
			name:    "no tag, with a port in the host",
			in:      "127.0.0.1:5050/fleet",
			wantErr: "has no tag",
		},
		{
			name:    "an empty tag",
			in:      "registry.example.com/fleet:",
			wantErr: `"" is not a valid tag`,
		},
		{
			name:    "a digest",
			in:      "registry.example.com/fleet@sha256:de5ef718a024560f857aad82cb95624067c13d0248455d498a25bea5cb9bc742",
			wantErr: "references by digest",
		},
		{
			name:    "a tag and a digest",
			in:      "registry.example.com/fleet:1.0.0@sha256:de5ef718a024560f857aad82cb95624067c13d0248455d498a25bea5cb9bc742",
			wantErr: "references by digest",
		},
		{
			name:    "another scheme",
			in:      "https://registry.example.com/fleet:1.0.0",
			wantErr: "only the oci:// scheme is accepted",
		},
		{
			name:    "a space inside",
			in:      "registry.example.com/fleet:1 0",
			wantErr: "forbidden characters",
		},
		{
			name:    "an invalid port",
			in:      "registry.example.com:abc/fleet:1.0.0",
			wantErr: `invalid port "abc"`,
		},
		{
			name:    "an empty path segment",
			in:      "registry.example.com/rancher//fleet:1.0.0",
			wantErr: `"" is not a valid repository name`,
		},
		{
			name:    "an upper case repository",
			in:      "registry.example.com/Fleet:1.0.0",
			wantErr: `"Fleet" is not a valid repository name`,
		},
		{
			name:    "a dot segment",
			in:      "registry.example.com/../fleet:1.0.0",
			wantErr: `".." is not a valid repository name`,
		},
		{
			name:    "a colon in the middle of the path",
			in:      "registry.example.com/a:b/fleet:1.0.0",
			wantErr: "only the tag may follow a ':'",
		},
		{
			name:    "a tag that starts with a dash",
			in:      "registry.example.com/fleet:-1",
			wantErr: `"-1" is not a valid tag`,
		},
		{
			name:    "a tag that is too long",
			in:      "registry.example.com/fleet:" + strings.Repeat("a", 129),
			wantErr: "is not a valid tag",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChartRef(tt.in)
			require.ErrorContains(t, err, "invalid chart reference")
			require.ErrorContains(t, err, tt.wantErr)
			require.Equal(t, ChartRef{}, got)
		})
	}
}

func TestParseChartRepo(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want ChartRepo
	}{
		{
			name: "host and one path segment",
			in:   "registry.example.com/fleet",
			want: ChartRepo{Host: "registry.example.com", Repository: "fleet"},
		},
		{
			name: "a nested repository",
			in:   "registry.suse.com/rancher/catalog/fleet",
			want: ChartRepo{Host: "registry.suse.com", Repository: "rancher/catalog/fleet"},
		},
		{
			name: "a port",
			in:   "127.0.0.1:5050/rancher/fleet",
			want: ChartRepo{Host: "127.0.0.1:5050", Repository: "rancher/fleet"},
		},
		{
			name: "the oci:// scheme",
			in:   "oci://registry.example.com/rancher/fleet",
			want: ChartRepo{Host: "registry.example.com", Repository: "rancher/fleet"},
		},
		{
			name: "space around the reference",
			in:   "  registry.example.com/rancher/fleet\n",
			want: ChartRepo{Host: "registry.example.com", Repository: "rancher/fleet"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChartRepo(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestChartRepoStringAndName(t *testing.T) {
	repo := ChartRepo{Host: "127.0.0.1:5050", Repository: "rancher/catalog/fleet"}

	require.Equal(t, "127.0.0.1:5050/rancher/catalog/fleet", repo.String())
	require.Equal(t, "fleet", repo.Name())
	require.Equal(t, "fleet", ChartRepo{Host: "h", Repository: "fleet"}.Name())
}

func TestParseChartRepoRejects(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{
			name:    "empty",
			in:      "",
			wantErr: "empty reference",
		},
		{
			name:    "only a host",
			in:      "registry.example.com",
			wantErr: "want host/path/chart",
		},
		{
			name:    "a host and a slash",
			in:      "registry.example.com/",
			wantErr: "want host/path/chart",
		},
		{
			name:    "a host and a port, no repository",
			in:      "127.0.0.1:5050",
			wantErr: "want host/path/chart",
		},
		{
			name:    "a tag",
			in:      "registry.example.com/rancher/fleet:110.0.3_up0.16.3",
			wantErr: "a tag is not accepted",
		},
		{
			name:    "a version with a plus",
			in:      "registry.example.com/rancher/fleet:110.0.3+up0.16.3",
			wantErr: "a tag is not accepted",
		},
		{
			name:    "a digest",
			in:      "registry.example.com/rancher/fleet@sha256:abc",
			wantErr: "a digest is not accepted",
		},
		{
			name:    "another scheme",
			in:      "https://registry.example.com/rancher/fleet",
			wantErr: "only the oci:// scheme is accepted",
		},
		{
			name:    "a space inside",
			in:      "registry.example.com/rancher /fleet",
			wantErr: "forbidden characters",
		},
		{
			name:    "an invalid port",
			in:      "registry.example.com:99999/fleet",
			wantErr: "invalid port",
		},
		{
			name:    "a trailing slash",
			in:      "registry.example.com/rancher/fleet/",
			wantErr: `"" is not a valid repository name`,
		},
		{
			name:    "an empty path segment",
			in:      "registry.example.com/rancher//fleet",
			wantErr: `"" is not a valid repository name`,
		},
		{
			name:    "an upper case repository",
			in:      "registry.example.com/Rancher/fleet",
			wantErr: `"Rancher" is not a valid repository name`,
		},
		{
			name:    "a dot segment",
			in:      "registry.example.com/rancher/../fleet",
			wantErr: `".." is not a valid repository name`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChartRepo(tt.in)
			require.ErrorContains(t, err, "invalid chart reference")
			require.ErrorContains(t, err, tt.wantErr)
			require.Equal(t, ChartRepo{}, got)
		})
	}
}
