package chart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetVersion(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		version string
		want    string
	}{
		{
			name:    "a plain value",
			in:      "apiVersion: v2\nname: fleet\nversion: 0.16.3\nappVersion: 0.16.3\n",
			version: "110.0.3+up0.16.3",
			want:    "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nappVersion: 0.16.3\n",
		},
		{
			name:    "a double quoted value",
			in:      "name: rancher-webhook\nversion: \"999\"\nappVersion: \"999\"\n",
			version: "111.0.4+up0.11.4",
			want:    "name: rancher-webhook\nversion: 111.0.4+up0.11.4\nappVersion: \"999\"\n",
		},
		{
			name:    "a single quoted value",
			in:      "name: a\nversion: '1.0'\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "a comment after the value is kept",
			in:      "name: a\nversion: 1.0.0   # set by the release\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion: 111.0.0+up1.0.0   # set by the release\n",
		},
		{
			name:    "the spacing after the colon is kept",
			in:      "name: a\nversion:    1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion:    111.0.0+up1.0.0\n",
		},
		{
			name:    "the first line of the file",
			in:      "version: 1.0.0\nname: a\n",
			version: "111.0.0+up1.0.0",
			want:    "version: 111.0.0+up1.0.0\nname: a\n",
		},
		{
			name:    "the last line, with no newline at the end",
			in:      "name: a\nversion: 1.0.0",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion: 111.0.0+up1.0.0",
		},
		{
			name:    "windows line endings are kept",
			in:      "name: a\r\nversion: 1.0.0\r\nappVersion: 1.0.0\r\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\r\nversion: 111.0.0+up1.0.0\r\nappVersion: 1.0.0\r\n",
		},
		{
			name:    "a version key under annotations is left alone",
			in:      "annotations:\n  version: keep\nname: a\nversion: 1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "annotations:\n  version: keep\nname: a\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "appVersion and kubeVersion are left alone",
			in:      "appVersion: 1.0.0\nkubeVersion: '>= 1.30.0-0'\nversion: 1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "appVersion: 1.0.0\nkubeVersion: '>= 1.30.0-0'\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "a block with a text after the key is left alone",
			in:      "description: |\n  version: not this one\nversion: 1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "description: |\n  version: not this one\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "the same version again",
			in:      "name: a\nversion: 111.0.0+up1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "a prerelease",
			in:      "name: a\nversion: 0.7.11-rc.3\n",
			version: "111.0.2+up0.7.11-rc.3",
			want:    "name: a\nversion: 111.0.2+up0.7.11-rc.3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetVersion([]byte(tt.in), tt.version)

			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
		})
	}
}

func TestSetVersionFails(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		version string
		wantErr string
	}{
		{
			name:    "no version",
			in:      "name: a\nappVersion: 1.0.0\n",
			version: "111.0.0+up1.0.0",
			wantErr: "the Chart.yaml has no version",
		},
		{
			name:    "a version only under annotations",
			in:      "name: a\nannotations:\n  version: x\n",
			version: "111.0.0+up1.0.0",
			wantErr: "the Chart.yaml has no version",
		},
		{
			name:    "empty",
			in:      "",
			version: "111.0.0+up1.0.0",
			wantErr: "the Chart.yaml has no version",
		},
		{
			name:    "two versions",
			in:      "version: 1.0.0\nname: a\nversion: 2.0.0\n",
			version: "111.0.0+up1.0.0",
			wantErr: "the Chart.yaml has more than one version",
		},
		{
			name:    "a block scalar",
			in:      "name: a\nversion: |\n  1.0.0\n",
			version: "111.0.0+up1.0.0",
			wantErr: "is not a value on one line",
		},
		{
			name:    "a value on the next line",
			in:      "name: a\nversion:\n  1.0.0\n",
			version: "111.0.0+up1.0.0",
			wantErr: "is not a value on one line",
		},
		{
			name:    "a version that breaks the YAML",
			in:      "name: a\nversion: 1.0.0\n",
			version: "1: 2",
			wantErr: "would not be valid YAML",
		},
		{
			name:    "a version that reads as something else",
			in:      "name: a\nversion: 1.0.0\n",
			version: "123",
			wantErr: `would not read "123" as the text of the version`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetVersion([]byte(tt.in), tt.version)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

// TestSetVersionOnRealCharts changes the version of the charts kept in
// tests/data and checks that the only difference is that line.
func TestSetVersionOnRealCharts(t *testing.T) {
	tests := []struct {
		name string
		path string
		from string
	}{
		{
			name: "the webhook chart as upstream keeps it, with a quoted placeholder",
			path: "rancher-webhook-upstream/src/rancher-webhook/Chart.yaml",
			from: `version: "999"`,
		},
		{
			name: "the compliance chart, with apiVersion v1",
			path: "rancher-compliance-upstream/src/rancher-compliance/Chart.yaml",
			from: "version: 1.5.4",
		},
		{
			name: "a released fleet chart",
			path: "fleet/src/fleet/Chart.yaml",
			from: "version: 110.0.3+up0.16.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "tests", "data", "charts", tt.path))
			require.NoError(t, err)
			original := string(data)
			require.Contains(t, original, "\n"+tt.from+"\n")

			got, err := SetVersion(data, "111.2.3+up9.8.7")

			require.NoError(t, err)
			want := replaceOnce(original, "\n"+tt.from+"\n", "\nversion: 111.2.3+up9.8.7\n")
			require.Equal(t, want, string(got))
		})
	}
}

func replaceOnce(s, old, replacement string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + replacement + s[i+len(old):]
		}
	}
	return s
}
