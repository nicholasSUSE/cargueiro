package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestPolicy(t *testing.T) *Policy {
	t.Helper()

	policy, err := NewPolicy([]string{"rc", "alpha", "beta", "rancher", "security"})
	require.NoError(t, err)
	return policy
}

func TestNewPolicyRejectsNoPrereleases(t *testing.T) {
	policy, err := NewPolicy(nil)

	require.EqualError(t, err, "version policy: no prereleases")
	require.Nil(t, policy)
}

func TestValidate(t *testing.T) {
	const standardMsg = `must look like 1XY.Z.W+upA.B.C, optionally followed by ` +
		`-rc.N, -alpha.N, -beta.N, -rancher.N or -security.N`

	tests := []struct {
		name    string
		version string
		plain   bool
		wantErr string
	}{
		// A standard chart.
		{
			name:    "a release",
			version: "110.0.3+up0.16.3",
		},
		{
			name:    "the lowest major",
			version: "100.0.0+up0.0.0",
		},
		{
			name:    "the highest major",
			version: "199.9.9+up9.9.9",
		},
		{
			name:    "many digits in the numbers",
			version: "110.12.345+up10.20.300",
		},
		{
			name:    "rc",
			version: "109.0.1+up0.10.4-rc.1",
		},
		{
			name:    "alpha",
			version: "109.0.1+up0.10.4-alpha.12",
		},
		{
			name:    "beta",
			version: "109.0.1+up0.10.4-beta.0",
		},
		{
			name:    "rancher",
			version: "109.0.1+up0.10.4-rancher.3",
		},
		{
			name:    "security",
			version: "109.0.1+up0.10.4-security.1",
		},

		// A standard chart, refused.
		{
			name:    "a major below 100",
			version: "99.0.0+up1.2.3",
			wantErr: standardMsg,
		},
		{
			name:    "a major above 199",
			version: "200.0.0+up1.2.3",
			wantErr: standardMsg,
		},
		{
			name:    "a four digit major",
			version: "1100.0.0+up1.2.3",
			wantErr: standardMsg,
		},
		{
			name:    "no +up part",
			version: "110.0.3",
			wantErr: standardMsg,
		},
		{
			name:    "a prerelease without the +up part",
			version: "109.0.1-rc.1",
			wantErr: standardMsg,
		},
		{
			name:    "a prerelease before the +up part",
			version: "109.0.1-rc.1+up0.10.4",
			wantErr: standardMsg,
		},
		{
			name:    "a prerelease without a dot",
			version: "109.0.1+up0.10.4-rc1",
			wantErr: standardMsg,
		},
		{
			name:    "a prerelease without a number",
			version: "109.0.1+up0.10.4-rc",
			wantErr: standardMsg,
		},
		{
			name:    "a prerelease number with a leading zero",
			version: "109.0.1+up0.10.4-rc.01",
			wantErr: standardMsg,
		},
		{
			name:    "an unknown prerelease",
			version: "109.0.1+up0.10.4-foo.1",
			wantErr: standardMsg,
		},
		{
			name:    "a prerelease that only starts with an allowed name",
			version: "109.0.1+up0.10.4-rcfoo.1",
			wantErr: standardMsg,
		},
		{
			name:    "uppercase prerelease",
			version: "109.0.1+up0.10.4-RC.1",
			wantErr: standardMsg,
		},
		{
			name:    "two prereleases",
			version: "109.0.1+up0.10.4-rc.1-beta.1",
			wantErr: standardMsg,
		},
		{
			name:    "more build metadata after the prerelease",
			version: "109.0.1+up0.10.4-rc.1+x",
			wantErr: standardMsg,
		},
		{
			name:    "a leading zero in the minor",
			version: "110.01.3+up0.16.3",
			wantErr: standardMsg,
		},
		{
			name:    "a leading zero in the upstream version",
			version: "110.0.3+up00.16.3",
			wantErr: standardMsg,
		},
		{
			name:    "an upstream version with two numbers",
			version: "110.0.3+up0.16",
			wantErr: standardMsg,
		},
		{
			name:    "a v in front",
			version: "v110.0.3+up0.16.3",
			wantErr: standardMsg,
		},
		{
			name:    "a space around it",
			version: " 110.0.3+up0.16.3 ",
			wantErr: standardMsg,
		},
		{
			name:    "the tag form",
			version: "110.0.3_up0.16.3",
			wantErr: standardMsg,
		},
		{
			name:    "empty",
			version: "",
			wantErr: standardMsg,
		},

		// A plain chart.
		{
			name:    "plain",
			version: "108.0.0",
			plain:   true,
		},
		{
			name:    "plain, many digits",
			version: "199.12.345",
			plain:   true,
		},
		{
			name:    "plain with a +up part",
			version: "108.0.0+up1.2.3",
			plain:   true,
			wantErr: "must look like 1XY.Z.W",
		},
		{
			name:    "plain with a prerelease",
			version: "108.0.0-rc.1",
			plain:   true,
			wantErr: "must look like 1XY.Z.W",
		},
		{
			name:    "plain with a major below 100",
			version: "99.0.0",
			plain:   true,
			wantErr: "must look like 1XY.Z.W",
		},
		{
			name:    "plain with a leading zero",
			version: "108.00.0",
			plain:   true,
			wantErr: "must look like 1XY.Z.W",
		},
		{
			name:    "plain, empty",
			version: "",
			plain:   true,
			wantErr: "must look like 1XY.Z.W",
		},
	}

	policy := newTestPolicy(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.Validate(tt.version, tt.plain)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, `version "`+tt.version+`"`)
		})
	}
}

func TestValidateUsesTheGivenPrereleases(t *testing.T) {
	policy, err := NewPolicy([]string{"rc"})
	require.NoError(t, err)

	require.NoError(t, policy.Validate("109.0.1+up0.10.4-rc.1", false))
	require.ErrorContains(t, policy.Validate("109.0.1+up0.10.4-beta.1", false),
		"optionally followed by -rc.N")
}

func TestValidateEscapesPrereleaseNames(t *testing.T) {
	policy, err := NewPolicy([]string{"r.c"})
	require.NoError(t, err)

	require.ErrorContains(t, policy.Validate("109.0.1+up0.10.4-rxc.1", false), "must look like")
}
