package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckURLs(t *testing.T) {
	const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name     string
		version  string
		url      string
		registry string
		wantErr  []string
	}{
		{
			name:    "the url of the staging registry of the edition",
			version: "111.0.3+up0.17.3",
			url:     goodURL,
		},
		{
			name:     "a registry given replaces it",
			version:  "111.0.3+up0.17.3",
			url:      "oci://127.0.0.1:5052/e2e/fleet:111.0.3_up0.17.3@" + digest,
			registry: "127.0.0.1:5052/e2e",
		},
		{
			name:    "another registry",
			version: "111.0.3+up0.17.3",
			url:     "oci://registry.example.com/rancher/catalog/fleet:111.0.3_up0.17.3@" + digest,
			wantErr: []string{"fleet 111.0.3+up0.17.3 has the url", "but it must be oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3@sha256:<digest>"},
		},
		{
			name:    "the registry of the other edition",
			version: "111.0.3+up0.17.3",
			url:     "oci://stgregistry.suse.com/rancher/charts/fleet:111.0.3_up0.17.3@" + digest,
			wantErr: []string{"but it must be oci://stgregistry.suse.com/rancher/catalog/fleet:"},
		},
		{
			name:    "another chart",
			version: "111.0.3+up0.17.3",
			url:     "oci://stgregistry.suse.com/rancher/catalog/fleet-crd:111.0.3_up0.17.3@" + digest,
			wantErr: []string{"fleet 111.0.3+up0.17.3 has the url"},
		},
		{
			name:    "the version as it is, with its +, in the tag",
			version: "111.0.3+up0.17.3",
			url:     "oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3+up0.17.3@" + digest,
			wantErr: []string{"but it must be oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3@"},
		},
		{
			name:    "another version",
			version: "111.0.3+up0.17.3",
			url:     "oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.4_up0.17.4@" + digest,
			wantErr: []string{"fleet 111.0.3+up0.17.3 has the url"},
		},
		{
			name:    "no digest",
			version: "111.0.3+up0.17.3",
			url:     "oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3",
			wantErr: []string{"fleet 111.0.3+up0.17.3 has the url"},
		},
		{
			name:    "a digest that is not sha256 of 64 hex digits",
			version: "111.0.3+up0.17.3",
			url:     "oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3@sha256:0123",
			wantErr: []string{"fleet 111.0.3+up0.17.3 has the url"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeIndex(t, fleetLine(tt.version, tt.url))

			report := Run(context.Background(), loadRules(t), Options{Dir: dir, Registry: tt.registry})

			result := report[0]
			if len(tt.wantErr) == 0 {
				require.True(t, result.Valid, result.Error)
				require.Equal(t, NoError, result.Error)
				return
			}
			require.False(t, result.Valid)
			for _, want := range tt.wantErr {
				require.Contains(t, result.Error, want)
			}
		})
	}
}

func TestCheckURLsNamesEveryVersionAtFault(t *testing.T) {
	const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	text := "apiVersion: v1\nentries:\n  fleet:\n" +
		"  - apiVersion: v2\n    name: fleet\n    version: 111.0.3+up0.17.3\n    urls:\n    - oci://evil.example.com/fleet:111.0.3_up0.17.3@" + digest + "\n" +
		"  - apiVersion: v2\n    name: fleet\n    version: 111.0.4+up0.17.4\n    urls:\n    - oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.4_up0.17.4@" + digest + "\n" +
		"  - apiVersion: v2\n    name: fleet\n    version: 111.0.5+up0.17.5\n    urls:\n    - oci://evil.example.com/fleet:111.0.5_up0.17.5@" + digest + "\n"

	report := Run(context.Background(), loadRules(t), Options{Dir: writeIndex(t, text)})

	require.False(t, report[0].Valid)
	require.Contains(t, report[0].Error, "fleet 111.0.3+up0.17.3")
	require.Contains(t, report[0].Error, "fleet 111.0.5+up0.17.5")
	require.NotContains(t, report[0].Error, "fleet 111.0.4+up0.17.4", "the version that is right is not named")
	require.Equal(t, 2, strings.Count(report[0].Error, "\n")+1, "one problem to a line")
	require.Contains(t, report[0].Description, "3 versions")
}

func TestCheckURLsAChartThatIsNotInTheRules(t *testing.T) {
	text := "apiVersion: v1\nentries:\n  unknown-chart:\n  - apiVersion: v2\n    name: unknown-chart\n    version: 1.0.0\n    urls:\n    - oci://stgregistry.suse.com/rancher/catalog/unknown-chart:1.0.0@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"

	report := Run(context.Background(), loadRules(t), Options{Dir: writeIndex(t, text)})

	require.False(t, report[0].Valid)
	require.Contains(t, report[0].Error, `unknown-chart 1.0.0: chart "unknown-chart" is not listed in the rules file`)
}
