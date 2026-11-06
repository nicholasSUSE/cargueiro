package chart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeChart writes the given files into a new directory and returns it. An
// empty content skips the file.
func writeChart(t *testing.T, chartYAML, valuesYAML string) string {
	t.Helper()
	dir := t.TempDir()
	if chartYAML != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYAML), 0o600))
	}
	if valuesYAML != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(valuesYAML), 0o600))
	}
	return dir
}

func TestLoad(t *testing.T) {
	dir := writeChart(t, `
name: fleet
version: 110.0.0
appVersion: 0.13.0
icon: https://example.com/icon.png
description: Fleet
kubeVersion: ">= 1.30.0-0 < 1.35.0-0"
dependencies:
  - name: sub
annotations:
  catalog.cattle.io/certified: rancher
  catalog.cattle.io/hidden: "true"
`, `
image:
  repository: rancher/fleet
  tag: v0.13.0
`)

	got, err := Load(dir)
	require.NoError(t, err)

	want := &Files{
		Metadata: Metadata{
			Name:        "fleet",
			Version:     "110.0.0",
			AppVersion:  "0.13.0",
			Icon:        "https://example.com/icon.png",
			Description: "Fleet",
			KubeVersion: ">= 1.30.0-0 < 1.35.0-0",
		},
		Annotations: map[string]string{
			"catalog.cattle.io/certified": "rancher",
			"catalog.cattle.io/hidden":    "true",
		},
		Values: map[string]any{
			"image": map[string]any{
				"repository": "rancher/fleet",
				"tag":        "v0.13.0",
			},
		},
	}
	require.Equal(t, want, got)
}

func TestLoadAcceptsEmptyFiles(t *testing.T) {
	dir := writeChart(t, "name: a\n", "# nothing here\n")

	got, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, "a", got.Metadata.Name)
	require.Nil(t, got.Annotations)
	require.Nil(t, got.Values)
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name    string
		chart   string
		values  string
		wantErr string
	}{
		{
			name:    "no Chart.yaml",
			values:  "a: 1\n",
			wantErr: "Chart.yaml not found",
		},
		{
			name:    "no values.yaml",
			chart:   "name: a\n",
			wantErr: "values.yaml not found",
		},
		{
			name:    "Chart.yaml is not yaml",
			chart:   "name: [\n",
			values:  "a: 1\n",
			wantErr: "parse",
		},
		{
			name:    "values.yaml is not yaml",
			chart:   "name: a\n",
			values:  "a: [\n",
			wantErr: "parse",
		},
		{
			name:    "values.yaml is a list",
			chart:   "name: a\n",
			values:  "- a\n- b\n",
			wantErr: "parse",
		},
		{
			name:    "annotations is a list",
			chart:   "annotations:\n  - a\n",
			values:  "a: 1\n",
			wantErr: "parse",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(writeChart(t, tt.chart, tt.values))
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestLoadMissingDirectory(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope"))
	require.ErrorContains(t, err, "Chart.yaml not found")
	require.Nil(t, got)
}
