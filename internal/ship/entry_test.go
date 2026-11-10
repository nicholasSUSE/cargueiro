package ship

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testURL  = "oci://stgregistry.suse.com/rancher/catalog/fleet:110.0.3+up0.16.3@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testIcon = "file://assets/logos/fleet.svg"
)

func TestBuildEntry(t *testing.T) {
	tests := []struct {
		name      string
		chartYAML string
		icon      string
		want      string
	}{
		{
			name:      "the icon changes and the url is added at the end",
			chartYAML: "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nicon: https://example.com/fleet.png\n",
			icon:      testIcon,
			want:      "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nicon: " + testIcon + "\nurls:\n- " + testURL + "\n",
		},
		{
			name:      "no icon to give leaves the icon as it is",
			chartYAML: "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nicon: https://example.com/fleet.png\n",
			icon:      "",
			want:      "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nicon: https://example.com/fleet.png\nurls:\n- " + testURL + "\n",
		},
		{
			name:      "a chart with no icon, which is exempt, is left with none",
			chartYAML: "apiVersion: v2\nname: fleet-crd\nversion: 110.0.3+up0.16.3\n",
			icon:      "",
			want:      "apiVersion: v2\nname: fleet-crd\nversion: 110.0.3+up0.16.3\nurls:\n- " + testURL + "\n",
		},
		{
			name:      "a file with no new line at the end",
			chartYAML: "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3",
			icon:      "",
			want:      "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nurls:\n- " + testURL + "\n",
		},
		{
			name:      "a file with Windows line endings gets the same",
			chartYAML: "apiVersion: v2\r\nname: fleet\r\nversion: 110.0.3+up0.16.3\r\n",
			icon:      "",
			want:      "apiVersion: v2\r\nname: fleet\r\nversion: 110.0.3+up0.16.3\r\nurls:\r\n- " + testURL + "\r\n",
		},
		{
			name:      "comments and the order of the keys are kept",
			chartYAML: "# the chart\nname: fleet\nannotations:\n  catalog.cattle.io/certified: rancher # yes\nversion: 110.0.3+up0.16.3\n",
			icon:      "",
			want:      "# the chart\nname: fleet\nannotations:\n  catalog.cattle.io/certified: rancher # yes\nversion: 110.0.3+up0.16.3\nurls:\n- " + testURL + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildEntry([]byte(tt.chartYAML), tt.icon, testURL)

			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
		})
	}
}

func TestBuildEntryFails(t *testing.T) {
	tests := []struct {
		name      string
		chartYAML string
		icon      string
		wantErr   string
	}{
		{
			name:      "an icon to set, and none in the chart",
			chartYAML: "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\n",
			icon:      testIcon,
			wantErr:   "the Chart.yaml has no icon",
		},
		{
			name:      "the chart has urls already",
			chartYAML: "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nurls:\n- https://example.com\n",
			wantErr:   "the Chart.yaml already has urls",
		},
		{
			name:      "a field the index does not know",
			chartYAML: "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nsomething: else\n",
			wantErr:   `unknown field "something"`,
		},
		{
			name:      "the file is not YAML",
			chartYAML: "name: [\n",
			wantErr:   "the entry would not be read by index",
		},
		{
			name:      "no name",
			chartYAML: "apiVersion: v2\nversion: 110.0.3+up0.16.3\n",
			wantErr:   "the entry would not be read by index: it has no name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildEntry([]byte(tt.chartYAML), tt.icon, testURL)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestBuildEntryOfARealChart(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "data", "charts", "rancher-compliance-upstream", "src", "rancher-compliance", "Chart.yaml"))
	require.NoError(t, err)
	original := string(data)
	const oldIcon = "icon: https://charts.rancher.io/assets/logos/rancher-compliance.svg\n"
	require.Contains(t, original, oldIcon)

	got, err := BuildEntry(data, "file://assets/logos/rancher-compliance.svg", testURL)

	require.NoError(t, err)
	want := replaceOnce(original, oldIcon, "icon: file://assets/logos/rancher-compliance.svg\n") + "urls:\n- " + testURL + "\n"
	require.Equal(t, want, string(got))
}

func replaceOnce(s, old, replacement string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + replacement + s[i+len(old):]
		}
	}
	return s
}
