package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckMajor(t *testing.T) {
	tests := []struct {
		name    string
		version string
		rancher string
		wantErr string
	}{
		{
			name:    "the major of Rancher 2.16",
			version: "111.0.3+up0.16.3",
			rancher: ">= 2.16.0-0 < 2.17.0-0",
		},
		{
			name:    "the major of Rancher 2.17",
			version: "112.4.0+up1.0.0-rc.1",
			rancher: ">= 2.17.0-0 < 2.18.0-0",
		},
		{
			name:    "a plain version",
			version: "111.0.0",
			rancher: ">= 2.16.0-0 < 2.17.0-0",
		},
		{
			name:    "space around the annotation",
			version: "111.0.3+up0.16.3",
			rancher: "  >=2.16.0-0 < 2.17.0-0 ",
		},
		{
			name:    "the upper bound does not decide",
			version: "111.0.3+up0.16.3",
			rancher: ">= 2.16.0-0 < 2.19.0-0",
		},
		{
			name:    "the major of another Rancher version",
			version: "112.0.3+up0.16.3",
			rancher: ">= 2.16.0-0 < 2.17.0-0",
			wantErr: `version "112.0.3+up0.16.3" has major 112, but charts for Rancher 2.16 have major 111`,
		},
		{
			name:    "a major below the list",
			version: "110.0.3+up0.16.3",
			rancher: ">= 2.16.0-0 < 2.17.0-0",
			wantErr: "has major 110, but charts for Rancher 2.16 have major 111",
		},
		{
			name:    "a Rancher version the rules do not know",
			version: "111.0.3+up0.16.3",
			rancher: ">= 2.20.0-0 < 2.21.0-0",
			wantErr: "no chart major for Rancher 2.20 in the rules file",
		},
		{
			name:    "an annotation without a lower bound",
			version: "111.0.3+up0.16.3",
			rancher: "< 2.17.0-0",
			wantErr: `rancher-version "< 2.17.0-0" has no lower bound like >= 2.16.0-0`,
		},
		{
			name:    "an empty annotation",
			version: "111.0.3+up0.16.3",
			rancher: "",
			wantErr: `rancher-version "" has no lower bound`,
		},
		{
			name:    "a version without a major",
			version: "latest",
			rancher: ">= 2.16.0-0 < 2.17.0-0",
			wantErr: `version "latest" does not start with a major`,
		},
		{
			name:    "an empty version",
			version: "",
			rancher: ">= 2.16.0-0 < 2.17.0-0",
			wantErr: `version "" does not start with a major`,
		},
		{
			name:    "a major that is too large",
			version: "99999999999999999999.0.0",
			rancher: ">= 2.16.0-0 < 2.17.0-0",
			wantErr: "has a major that is too large",
		},
	}

	policy := newTestPolicy(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.CheckMajor(tt.version, tt.rancher)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestRancherOf(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		want       string
		wantOK     bool
	}{
		{
			name:       "a range",
			annotation: ">= 2.16.0-0 < 2.17.0-0",
			want:       "2.16",
			wantOK:     true,
		},
		{
			name:       "no space after the operator, and space around",
			annotation: "  >=2.15.1 < 2.17.0-0 ",
			want:       "2.15",
			wantOK:     true,
		},
		{
			name:       "an upper bound only",
			annotation: "< 2.17.0-0",
			want:       "",
			wantOK:     false,
		},
		{
			name:       "empty",
			annotation: "",
			want:       "",
			wantOK:     false,
		},
		{
			name:       "not a Rancher 2 version",
			annotation: ">= 3.1.0-0",
			want:       "",
			wantOK:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := RancherOf(tt.annotation)

			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSupportedRancher(t *testing.T) {
	tests := []struct {
		name    string
		rancher string
		want    bool
	}{
		{
			name:    "the lowest supported version",
			rancher: "2.16",
			want:    true,
		},
		{
			name:    "a later version",
			rancher: "2.19",
			want:    true,
		},
		{
			name:    "a minor with more digits",
			rancher: "2.120",
			want:    true,
		},
		{
			name:    "the version before",
			rancher: "2.15",
			want:    false,
		},
		{
			name:    "an old version",
			rancher: "2.9",
			want:    false,
		},
		{
			name:    "not Rancher 2",
			rancher: "3.20",
			want:    false,
		},
		{
			name:    "no minor",
			rancher: "2",
			want:    false,
		},
		{
			name:    "a minor that is not a number",
			rancher: "2.x",
			want:    false,
		},
		{
			name:    "empty",
			rancher: "",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, SupportedRancher(tt.rancher))
		})
	}
}
