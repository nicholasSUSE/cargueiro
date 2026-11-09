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
