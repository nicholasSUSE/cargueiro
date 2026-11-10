package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNext(t *testing.T) {
	tests := []struct {
		name      string
		published []string
		major     int
		upstream  string
		plain     bool
		want      string
	}{
		{
			name:      "nothing is published: the first version",
			published: nil,
			major:     111,
			upstream:  "1.0.0",
			want:      "111.0.0+up1.0.0",
		},
		{
			name:      "a new upstream version: W goes up by one",
			published: []string{"111.0.0+up1.0.0", "111.0.1+up1.0.0"},
			major:     111,
			upstream:  "1.0.1",
			want:      "111.0.2+up1.0.1",
		},
		{
			name:      "the first candidate of a new upstream version",
			published: []string{"111.0.1+up1.0.0"},
			major:     111,
			upstream:  "1.0.1-rc.1",
			want:      "111.0.2+up1.0.1-rc.1",
		},
		{
			name:      "the second candidate keeps Z.W",
			published: []string{"111.0.1+up1.0.0", "111.0.2+up1.0.1-rc.1"},
			major:     111,
			upstream:  "1.0.1-rc.2",
			want:      "111.0.2+up1.0.1-rc.2",
		},
		{
			name:      "the final keeps Z.W of its candidates",
			published: []string{"111.0.1+up1.0.0", "111.0.2+up1.0.1-rc.1", "111.0.2+up1.0.1-rc.2"},
			major:     111,
			upstream:  "1.0.1",
			want:      "111.0.2+up1.0.1",
		},
		{
			name:      "a final of the same upstream is published: a re-release gets the next W",
			published: []string{"111.0.2+up1.0.1"},
			major:     111,
			upstream:  "1.0.1",
			want:      "111.0.3+up1.0.1",
		},
		{
			name:      "a candidate after the final of its upstream version",
			published: []string{"111.0.2+up1.0.1-rc.1", "111.0.2+up1.0.1"},
			major:     111,
			upstream:  "1.0.1-rc.9",
			want:      "111.0.3+up1.0.1-rc.9",
		},
		{
			name:      "candidates of another upstream version do not count",
			published: []string{"111.0.2+up1.0.0-rc.1"},
			major:     111,
			upstream:  "1.0.1-rc.1",
			want:      "111.0.3+up1.0.1-rc.1",
		},
		{
			name:      "other majors are left out",
			published: []string{"110.5.9+up0.9.0", "112.0.0+up2.0.0", "112.0.1+up2.0.1-rc.1"},
			major:     111,
			upstream:  "1.0.0",
			want:      "111.0.0+up1.0.0",
		},
		{
			name:      "the highest Z.W wins, whatever the order given",
			published: []string{"111.0.3+up1.0.2", "111.1.0+up1.1.0", "111.0.9+up1.0.9"},
			major:     111,
			upstream:  "1.1.1",
			want:      "111.1.1+up1.1.1",
		},
		{
			name:      "W goes past nine with no roll-over",
			published: []string{"111.0.9+up1.0.9"},
			major:     111,
			upstream:  "1.0.10",
			want:      "111.0.10+up1.0.10",
		},
		{
			name:      "W is twelve",
			published: []string{"111.0.12+up1.0.12", "111.0.11+up1.0.11"},
			major:     111,
			upstream:  "1.0.13",
			want:      "111.0.13+up1.0.13",
		},
		{
			name:      "a line with a Z above zero",
			published: []string{"111.2.3+up2.0.0"},
			major:     111,
			upstream:  "2.0.1",
			want:      "111.2.4+up2.0.1",
		},
		{
			name:      "a prerelease name other than rc",
			published: []string{"111.0.0+up1.0.0"},
			major:     111,
			upstream:  "1.0.1-rancher.1",
			want:      "111.0.1+up1.0.1-rancher.1",
		},
		{
			name:      "a plain chart, nothing published",
			published: nil,
			major:     111,
			plain:     true,
			want:      "111.0.0",
		},
		{
			name:      "a plain chart, W goes up by one",
			published: []string{"111.0.9", "110.7.7", "112.0.0"},
			major:     111,
			plain:     true,
			want:      "111.0.10",
		},
	}

	policy := newTestPolicy(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := policy.Next(tt.published, tt.major, tt.upstream, tt.plain)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNextFails(t *testing.T) {
	tests := []struct {
		name      string
		published []string
		upstream  string
		plain     bool
		wantErr   string
	}{
		{
			name:      "the same candidate twice",
			published: []string{"111.0.2+up1.0.1-rc.1"},
			upstream:  "1.0.1-rc.1",
			wantErr:   "version 111.0.2+up1.0.1-rc.1 is already published",
		},
		{
			name:      "a published tag that is not a version",
			published: []string{"111.0.0+up1.0.0", "latest"},
			upstream:  "1.0.1",
			wantErr:   `version "latest" must look like`,
		},
		{
			name:      "a published version of another major that is not a version",
			published: []string{"110.0.0"},
			upstream:  "1.0.1",
			wantErr:   `version "110.0.0" must look like`,
		},
		{
			name:     "an upstream version that is a placeholder",
			upstream: "999",
			wantErr:  `the upstream version "999" does not make a valid version`,
		},
		{
			name:     "an upstream version with a v",
			upstream: "v1.0.1",
			wantErr:  `the upstream version "v1.0.1" does not make a valid version`,
		},
		{
			name:     "an upstream version with a prerelease that is not allowed",
			upstream: "1.0.1-dev.1",
			wantErr:  `the upstream version "1.0.1-dev.1" does not make a valid version`,
		},
		{
			name:     "empty",
			upstream: "",
			wantErr:  `the upstream version "" does not make a valid version`,
		},
		{
			name:      "a plain chart with a version that has a +up part",
			published: []string{"111.0.0+up1.0.0"},
			plain:     true,
			wantErr:   `version "111.0.0+up1.0.0" must look like 1XY.Z.W`,
		},
	}

	policy := newTestPolicy(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := policy.Next(tt.published, 111, tt.upstream, tt.plain)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, got)
		})
	}
}
