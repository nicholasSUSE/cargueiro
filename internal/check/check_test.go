package check

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
)

const rulesFile = `
kinds:
  main:
    required: [certified]
families:
  - family: fleet
    charts:
      - { name: fleet, kind: main }
`

func parseRules(t *testing.T) *rules.Config {
	t.Helper()
	cfg, err := rules.Parse([]byte(rulesFile))
	require.NoError(t, err)
	return cfg
}

func TestChart(t *testing.T) {
	const certified = "catalog.cattle.io/certified"
	images := map[string]any{
		"image": map[string]any{
			"repository": "rancher/fleet",
			"tag":        "v1",
		},
	}

	tests := []struct {
		name            string
		files           *chart.Files
		wantValid       bool
		wantAnnotations []string
		wantHIP15       []string
	}{
		{
			name: "valid",
			files: &chart.Files{
				Metadata: chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{
					certified: "rancher",
				},
			},
			wantValid:       true,
			wantAnnotations: []string{},
			wantHIP15:       []string{},
		},
		{
			name: "annotation errors only",
			files: &chart.Files{
				Metadata:    chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{},
			},
			wantAnnotations: []string{"missing annotation catalog.cattle.io/certified"},
			wantHIP15:       []string{},
		},
		{
			name: "helm images errors only",
			files: &chart.Files{
				Metadata: chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{
					certified: "rancher",
				},
				Values: images,
			},
			wantAnnotations: []string{},
			wantHIP15: []string{
				`values.yaml image: image "rancher/fleet:v1" is not listed in helm.sh/images`,
			},
		},
		{
			name: "both",
			files: &chart.Files{
				Metadata:    chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{},
				Values:      images,
			},
			wantAnnotations: []string{"missing annotation catalog.cattle.io/certified"},
			wantHIP15: []string{
				`values.yaml image: image "rancher/fleet:v1" is not listed in helm.sh/images`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Chart(parseRules(t), tt.files)
			require.NoError(t, err)

			require.Equal(t, "fleet", got.Chart)
			require.Equal(t, "main", got.Kind)
			require.Equal(t, "fleet", got.Family)
			require.Equal(t, "fleet", got.Metadata.Name)
			require.Equal(t, tt.wantValid, got.Valid)
			require.Equal(t, tt.wantAnnotations, got.Annotations.Errors)
			require.Equal(t, len(tt.wantAnnotations) == 0, got.Annotations.Valid)
			require.Equal(t, tt.wantHIP15, got.HIP15Annotations.Errors)
			require.Equal(t, len(tt.wantHIP15) == 0, got.HIP15Annotations.Valid)
		})
	}
}

func TestChartNotListed(t *testing.T) {
	tests := []struct {
		name  string
		chart string
	}{
		{
			name:  "a chart that is not in the file",
			chart: "epinio",
		},
		{
			name:  "no name in Chart.yaml",
			chart: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := &chart.Files{Metadata: chart.Metadata{Name: tt.chart}}
			got, err := Chart(parseRules(t), files)
			require.ErrorContains(t, err, "is not listed in the rules file")
			require.Nil(t, got)
		})
	}
}

func TestResultJSON(t *testing.T) {
	files := &chart.Files{
		Metadata: chart.Metadata{Name: "fleet"},
		Annotations: map[string]string{
			"catalog.cattle.io/certified": "rancher",
		},
	}
	got, err := Chart(parseRules(t), files)
	require.NoError(t, err)

	data, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"chart": "fleet",
		"kind": "main",
		"family": "fleet",
		"valid": true,
		"metadata": {
			"name": "fleet",
			"version": "",
			"appVersion": "",
			"icon": "",
			"description": "",
			"kubeVersion": ""
		},
		"annotations": {"valid": true, "errors": []},
		"hip15-annotations": {"valid": true, "errors": []}
	}`, string(data))
}

// TestChartKubeVersion makes sure the result reports the kube-version
// annotation, and not Helm's kubeVersion field, which Chart.yaml may lack.
func TestChartKubeVersion(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{
			name: "the annotation",
			annotations: map[string]string{
				"catalog.cattle.io/kube-version": ">= 1.34.0-0 < 1.37.0-0",
			},
			want: ">= 1.34.0-0 < 1.37.0-0",
		},
		{
			name: "space around the annotation is ignored",
			annotations: map[string]string{
				"catalog.cattle.io/kube-version": "  >= 1.34.0-0 < 1.37.0-0 ",
			},
			want: ">= 1.34.0-0 < 1.37.0-0",
		},
		{
			name:        "no annotation",
			annotations: map[string]string{},
			want:        "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := &chart.Files{
				Metadata:    chart.Metadata{Name: "fleet"},
				Annotations: tt.annotations,
			}
			got, err := Chart(parseRules(t), files)
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Metadata.KubeVersion)
		})
	}
}

func TestResultFails(t *testing.T) {
	tests := []struct {
		name        string
		annotations bool
		hip15       bool
		ignoreHIP15 bool
		wantFails   bool
	}{
		{
			name:        "valid",
			annotations: true,
			hip15:       true,
			wantFails:   false,
		},
		{
			name:        "valid, HIP-15 ignored",
			annotations: true,
			hip15:       true,
			ignoreHIP15: true,
			wantFails:   false,
		},
		{
			name:        "annotation errors",
			annotations: false,
			hip15:       true,
			wantFails:   true,
		},
		{
			name:        "annotation errors, HIP-15 ignored",
			annotations: false,
			hip15:       true,
			ignoreHIP15: true,
			wantFails:   true,
		},
		{
			name:        "HIP-15 errors",
			annotations: true,
			hip15:       false,
			wantFails:   true,
		},
		{
			name:        "HIP-15 errors, HIP-15 ignored",
			annotations: true,
			hip15:       false,
			ignoreHIP15: true,
			wantFails:   false,
		},
		{
			name:        "both, HIP-15 ignored",
			annotations: false,
			hip15:       false,
			ignoreHIP15: true,
			wantFails:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{
				Valid:            tt.annotations && tt.hip15,
				Annotations:      Section{Valid: tt.annotations},
				HIP15Annotations: Section{Valid: tt.hip15},
			}
			require.Equal(t, tt.wantFails, result.Fails(tt.ignoreHIP15))
		})
	}
}
