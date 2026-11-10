package version

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortNewestFirst(t *testing.T) {
	tests := []struct {
		name  string
		in    []string
		plain bool
		want  []string
	}{
		{
			name: "1XY.Z.W is compared as numbers, not as text",
			in: []string{
				"110.0.3+up1.0.0",
				"110.0.10+up1.0.0",
				"109.9.9+up1.0.0",
				"110.1.0+up1.0.0",
				"111.0.0+up1.0.0",
				"110.0.9+up1.0.0",
			},
			want: []string{
				"111.0.0+up1.0.0",
				"110.1.0+up1.0.0",
				"110.0.10+up1.0.0",
				"110.0.9+up1.0.0",
				"110.0.3+up1.0.0",
				"109.9.9+up1.0.0",
			},
		},
		{
			name: "the same 1XY.Z.W falls back to the upstream version, as numbers",
			in: []string{
				"110.0.3+up0.9.0",
				"110.0.3+up1.0.0",
				"110.0.3+up0.16.10",
				"110.0.3+up0.16.3",
			},
			want: []string{
				"110.0.3+up1.0.0",
				"110.0.3+up0.16.10",
				"110.0.3+up0.16.3",
				"110.0.3+up0.9.0",
			},
		},
		{
			name: "1XY.Z.W decides before the upstream version",
			in: []string{
				"110.0.3+up9.9.9",
				"110.0.4+up0.0.1",
			},
			want: []string{
				"110.0.4+up0.0.1",
				"110.0.3+up9.9.9",
			},
		},
		{
			name: "a release is newer than its prereleases, which follow the rank of their names",
			in: []string{
				"110.0.3+up0.16.3-alpha.1",
				"110.0.3+up0.16.3-rc.1",
				"110.0.3+up0.16.3-beta.1",
				"110.0.3+up0.16.3",
				"110.0.3+up0.16.3-rancher.1",
			},
			want: []string{
				"110.0.3+up0.16.3",
				"110.0.3+up0.16.3-rancher.1",
				"110.0.3+up0.16.3-rc.1",
				"110.0.3+up0.16.3-beta.1",
				"110.0.3+up0.16.3-alpha.1",
			},
		},
		{
			name: "the number of a prerelease is compared as a number",
			in: []string{
				"110.0.3+up0.16.3-rc.2",
				"110.0.3+up0.16.3-rc.10",
				"110.0.3+up0.16.3-rc.1",
			},
			want: []string{
				"110.0.3+up0.16.3-rc.10",
				"110.0.3+up0.16.3-rc.2",
				"110.0.3+up0.16.3-rc.1",
			},
		},
		{
			name: "the name decides before the number",
			in: []string{
				"110.0.3+up0.16.3-alpha.9",
				"110.0.3+up0.16.3-beta.1",
			},
			want: []string{
				"110.0.3+up0.16.3-beta.1",
				"110.0.3+up0.16.3-alpha.9",
			},
		},
		{
			name: "the upstream version decides before the prerelease",
			in: []string{
				"110.0.3+up0.16.3",
				"110.0.3+up0.17.0-alpha.1",
				"110.0.3+up0.16.4-rc.1",
			},
			want: []string{
				"110.0.3+up0.17.0-alpha.1",
				"110.0.3+up0.16.4-rc.1",
				"110.0.3+up0.16.3",
			},
		},
		{
			name: "plain charts are compared by 1XY.Z.W",
			in: []string{
				"110.0.9",
				"111.0.0",
				"110.0.10",
			},
			plain: true,
			want: []string{
				"111.0.0",
				"110.0.10",
				"110.0.9",
			},
		},
		{
			name: "the same version twice stays twice",
			in: []string{
				"110.0.3+up0.16.3",
				"110.0.3+up0.16.3",
			},
			want: []string{
				"110.0.3+up0.16.3",
				"110.0.3+up0.16.3",
			},
		},
		{
			name: "one version",
			in:   []string{"110.0.3+up0.16.3"},
			want: []string{"110.0.3+up0.16.3"},
		},
		{
			name: "no versions",
			in:   nil,
			want: []string{},
		},
	}

	policy := newTestPolicy(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := append([]string(nil), tt.in...)

			got, err := policy.SortNewestFirst(tt.in, tt.plain)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, before, append([]string(nil), tt.in...), "the input is not changed")
		})
	}
}

func TestSortNewestFirstFollowsTheRankItIsGiven(t *testing.T) {
	versions := []string{
		"110.0.3+up0.16.3-rc.1",
		"110.0.3+up0.16.3-alpha.1",
	}

	tests := []struct {
		name        string
		prereleases []string
		want        []string
	}{
		{
			name:        "rc is newer than alpha",
			prereleases: []string{"rc", "alpha"},
			want:        []string{"110.0.3+up0.16.3-rc.1", "110.0.3+up0.16.3-alpha.1"},
		},
		{
			name:        "alpha is newer than rc",
			prereleases: []string{"alpha", "rc"},
			want:        []string{"110.0.3+up0.16.3-alpha.1", "110.0.3+up0.16.3-rc.1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := NewPolicy(tt.prereleases, map[string]int{"2.16": 111})
			require.NoError(t, err)

			got, err := policy.SortNewestFirst(versions, false)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSortNewestFirstFails(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		plain   bool
		wantErr string
	}{
		{
			name:    "a tag that is not a version",
			in:      []string{"110.0.3+up0.16.3", "latest"},
			wantErr: `version "latest" must look like`,
		},
		{
			name:    "a version without the +up part",
			in:      []string{"110.0.3"},
			wantErr: `version "110.0.3" must look like`,
		},
		{
			name:    "a prerelease that is not allowed",
			in:      []string{"110.0.3+up0.16.3-foo.1"},
			wantErr: `version "110.0.3+up0.16.3-foo.1" must look like`,
		},
		{
			name:    "the tag form",
			in:      []string{"110.0.3_up0.16.3"},
			wantErr: `version "110.0.3_up0.16.3" must look like`,
		},
		{
			name:    "a plain chart with a +up part",
			in:      []string{"110.0.3+up0.16.3"},
			plain:   true,
			wantErr: `version "110.0.3+up0.16.3" must look like 1XY.Z.W`,
		},
		{
			name:    "a prerelease number that is too large",
			in:      []string{"110.0.3+up0.16.3-rc.99999999999999999999"},
			wantErr: "has a prerelease number that is too large",
		},
	}

	policy := newTestPolicy(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := policy.SortNewestFirst(tt.in, tt.plain)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

// TestSortNewestFirstAgreesWithChartsBuildScripts holds the cases that
// charts-build-scripts tests its own sort with (SortUpstreamAppVersions), so
// that both put the versions of a chart in the same order. The cases of that
// test that are not Rancher chart versions (no +up part, or no 1XY) are left
// out, because here they are refused.
func TestSortNewestFirstAgreesWithChartsBuildScripts(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{
			name: "stable versions only",
			want: []string{"108.0.2+up0.9.2", "108.0.1+up0.9.1", "108.0.0+up0.9.0"},
		},
		{
			name: "stable and rcs with the same base",
			want: []string{"108.0.0+up0.9.0", "108.0.0+up0.9.0-rc.2", "108.0.0+up0.9.0-rc.1"},
		},
		{
			name: "rcs only",
			want: []string{"108.0.0+up0.9.0-rc.3", "108.0.0+up0.9.0-rc.2", "108.0.0+up0.9.0-rc.1"},
		},
		{
			name: "mixed bases with rcs",
			want: []string{
				"109.0.0+up0.10.0",
				"109.0.0+up0.10.0-rc.1",
				"108.0.0+up0.9.0",
				"108.0.0+up0.9.0-rc.1",
			},
		},
		{
			name: "stable, rc, beta and alpha",
			want: []string{
				"108.0.0+up0.14.0",
				"108.0.0+up0.14.0-rc.2",
				"108.0.0+up0.14.0-rc.1",
				"108.0.0+up0.14.0-beta.1",
				"108.0.0+up0.14.0-alpha.5",
				"108.0.0+up0.14.0-alpha.4",
				"108.0.0+up0.14.0-alpha.3",
				"108.0.0+up0.14.0-alpha.2",
				"108.0.0+up0.14.0-alpha.1",
			},
		},
		{
			name: "the same 1XY.Z.W with different upstream versions",
			want: []string{
				"109.0.1+up0.10.4-rc.1",
				"109.0.1+up0.10.1-rc.2",
				"109.0.1+up0.10.1-rc.1",
				"109.0.0+up0.10.0",
			},
		},
		{
			name: "the same upstream version with every prerelease name",
			want: []string{
				"109.0.1+up0.10.1",
				"109.0.1+up0.10.1-rc.1",
				"109.0.1+up0.10.1-beta.1",
				"109.0.1+up0.10.1-alpha.1",
			},
		},
		{
			name: "upstream major changes",
			want: []string{"109.0.1+up2.0.0", "109.0.1+up1.5.0", "109.0.1+up1.0.0"},
		},
	}

	policy := newTestPolicy(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := policy.SortNewestFirst(reversed(tt.want), false)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestSortNewestFirstDoesNotDependOnTheOrderGiven shuffles the versions many
// times. Helm's own sort gives different orders for versions that differ only
// after the +, so this is what the index must not do.
func TestSortNewestFirstDoesNotDependOnTheOrderGiven(t *testing.T) {
	want := []string{
		"111.0.4+up0.17.4-rc.10",
		"111.0.4+up0.17.4-rc.2",
		"111.0.4+up0.17.4-rc.1",
		"111.0.3+up0.17.4",
		"111.0.3+up0.17.3",
		"111.0.3+up0.17.2",
		"111.0.2+up0.17.2",
	}

	policy := newTestPolicy(t)
	random := rand.New(rand.NewPCG(1, 2))
	for range 200 {
		in := slices.Clone(want)
		random.Shuffle(len(in), func(i, j int) { in[i], in[j] = in[j], in[i] })

		got, err := policy.SortNewestFirst(in, false)

		require.NoError(t, err)
		require.Equal(t, want, got, "given %v", in)
	}
}

func reversed(s []string) []string {
	r := slices.Clone(s)
	slices.Reverse(r)
	return r
}
