package index

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/require"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// archiveOf returns a chart archive that holds the given Chart.yaml.
func archiveOf(t *testing.T, chartYAML string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "fleet/Chart.yaml",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(chartYAML)),
	}))
	_, err := tw.Write([]byte(chartYAML))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

const fleetChartYAML = `apiVersion: v2
name: fleet
version: 111.0.3+up0.17.3
appVersion: 0.17.3
description: Fleet Controller
icon: https://charts.rancher.io/assets/logos/fleet.svg
annotations:
  catalog.cattle.io/certified: rancher
`

// fleetEntry is the entry that fleetChartYAML gives, once ship has changed the
// icon and added the URL.
func fleetEntry() *repo.ChartVersion {
	return &repo.ChartVersion{
		Metadata: &chart.Metadata{
			APIVersion:  "v2",
			Name:        "fleet",
			Version:     "111.0.3+up0.17.3",
			AppVersion:  "0.17.3",
			Description: "Fleet Controller",
			Icon:        "file://assets/logos/fleet.svg",
			Annotations: map[string]string{"catalog.cattle.io/certified": "rancher"},
		},
		URLs: []string{"oci://registry.example.com/fleet:111.0.3+up0.17.3@" + testDigest},
	}
}

func TestCheckArchive(t *testing.T) {
	tests := []struct {
		name       string
		chartYAML  string
		entryEdit  func(*repo.ChartVersion)
		wantFields string
	}{
		{
			name:      "the chart the entry describes, with the icon ship changed",
			chartYAML: fleetChartYAML,
		},
		{
			name:      "an archive with no icon",
			chartYAML: "apiVersion: v2\nname: fleet\nversion: 111.0.3+up0.17.3\nappVersion: 0.17.3\ndescription: Fleet Controller\nannotations:\n  catalog.cattle.io/certified: rancher\n",
		},
		{
			name:      "an entry with no icon, as a crd chart has",
			chartYAML: fleetChartYAML,
			entryEdit: func(e *repo.ChartVersion) { e.Icon = "" },
		},
		{
			name:       "another version",
			chartYAML:  fleetChartYAML,
			entryEdit:  func(e *repo.ChartVersion) { e.Version = "111.0.4+up0.17.4" },
			wantFields: "version",
		},
		{
			name:       "another name",
			chartYAML:  fleetChartYAML,
			entryEdit:  func(e *repo.ChartVersion) { e.Name = "fleet-crd" },
			wantFields: "name",
		},
		{
			name:       "another description",
			chartYAML:  fleetChartYAML,
			entryEdit:  func(e *repo.ChartVersion) { e.Description = "Another" },
			wantFields: "description",
		},
		{
			name:      "another annotation",
			chartYAML: fleetChartYAML,
			entryEdit: func(e *repo.ChartVersion) {
				e.Annotations["catalog.cattle.io/certified"] = "partner"
			},
			wantFields: "annotations",
		},
		{
			name:      "an annotation the archive has and the entry has not",
			chartYAML: fleetChartYAML,
			entryEdit: func(e *repo.ChartVersion) {
				e.Annotations = nil
			},
			wantFields: "annotations",
		},
		{
			name:       "a field only the archive has",
			chartYAML:  fleetChartYAML + "kubeVersion: '>= 1.30.0-0'\n",
			wantFields: "kubeVersion",
		},
		{
			name:       "a field only the entry has",
			chartYAML:  fleetChartYAML,
			entryEdit:  func(e *repo.ChartVersion) { e.KubeVersion = ">= 1.30.0-0" },
			wantFields: "kubeVersion",
		},
		{
			name:      "several fields",
			chartYAML: fleetChartYAML,
			entryEdit: func(e *repo.ChartVersion) {
				e.AppVersion = "9.9.9"
				e.Description = "Another"
			},
			wantFields: "appVersion, description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := fleetEntry()
			if tt.entryEdit != nil {
				tt.entryEdit(entry)
			}

			err := checkArchive(entry, archiveOf(t, tt.chartYAML))

			if tt.wantFields == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, entry.Name+" "+entry.Version+": the Chart.yaml in the archive differs from the entry in: "+tt.wantFields)
		})
	}
}

func TestCheckArchiveFails(t *testing.T) {
	tests := []struct {
		name    string
		archive []byte
		wantErr string
	}{
		{
			name:    "not an archive",
			archive: []byte("this is not an archive"),
			wantErr: "fleet 111.0.3+up0.17.3: read the archive: the archive is not gzip",
		},
		{
			name:    "a Chart.yaml that is not YAML",
			archive: archiveOf(t, "name: [\n"),
			wantErr: "fleet 111.0.3+up0.17.3: read the Chart.yaml in the archive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkArchive(fleetEntry(), tt.archive)

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
