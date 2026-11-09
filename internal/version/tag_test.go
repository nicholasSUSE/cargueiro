package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTagConversion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		tag     string
	}{
		{
			name:    "build metadata",
			version: "110.0.3+up0.16.3",
			tag:     "110.0.3_up0.16.3",
		},
		{
			name:    "build metadata with a prerelease",
			version: "109.0.1+up0.10.4-rc.1",
			tag:     "109.0.1_up0.10.4-rc.1",
		},
		{
			name:    "no build metadata",
			version: "110.0.3",
			tag:     "110.0.3",
		},
		{
			name:    "empty",
			version: "",
			tag:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.tag, ToOCITag(tt.version))
			require.Equal(t, tt.version, FromOCITag(tt.tag))
		})
	}
}

func TestToOCITagLeavesATagAlone(t *testing.T) {
	require.Equal(t, "110.0.3_up0.16.3", ToOCITag("110.0.3_up0.16.3"))
}

func TestFromOCITagLeavesAVersionAlone(t *testing.T) {
	require.Equal(t, "110.0.3+up0.16.3", FromOCITag("110.0.3+up0.16.3"))
}
