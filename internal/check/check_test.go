package check

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/rules"
)

const rulesFile = `
kinds:
  main:
    required: [certified]
  crd: {}
families:
  - family: fleet
    charts:
      - { name: fleet, kind: main, edition: rancher-charts }
      - { name: fleet-crd, kind: crd, edition: rancher-charts }
  - family: system-upgrade-controller
    charts:
      - { name: system-upgrade-controller, kind: main, edition: rancher-charts, version: plain }
registries:
  rancher-charts:
    charts-staging: stgregistry.suse.com/rancher/catalog
    charts-production: registry.suse.com/rancher/catalog
  prime-charts:
    charts-staging: stgregistry.suse.com/rancher/charts
    charts-production: registry.suse.com/rancher/charts
  images:
    staging: stgregistry.suse.com/rancher
    production: registry.suse.com/rancher
versions:
  prereleases: [rancher, rc, beta, alpha]
  majors:
    "2.16": 111
    "2.17": 112
`

// goodVersion is a valid version for fleet when the rules tie 2.16 to 111.
const goodVersion = "111.0.3+up0.16.3"

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
		version         string
		wantAnnotations []string
		wantHIP15       []string
		wantVersion     []string
	}{
		{
			name: "valid",
			files: &chart.Files{
				Metadata: chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{
					certified: "rancher",
				},
			},
			version:         goodVersion,
			wantValid:       true,
			wantAnnotations: []string{},
			wantHIP15:       []string{},
			wantVersion:     []string{},
		},
		{
			name: "annotation errors only",
			files: &chart.Files{
				Metadata:    chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{},
			},
			version:         goodVersion,
			wantAnnotations: []string{"missing annotation catalog.cattle.io/certified"},
			wantHIP15:       []string{},
			wantVersion:     []string{},
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
			version:         goodVersion,
			wantAnnotations: []string{},
			wantHIP15: []string{
				`values.yaml image: image "rancher/fleet:v1" is not listed in helm.sh/images`,
			},
			wantVersion: []string{},
		},
		{
			name: "both",
			files: &chart.Files{
				Metadata:    chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{},
				Values:      images,
			},
			version:         goodVersion,
			wantAnnotations: []string{"missing annotation catalog.cattle.io/certified"},
			wantHIP15: []string{
				`values.yaml image: image "rancher/fleet:v1" is not listed in helm.sh/images`,
			},
			wantVersion: []string{},
		},
		{
			name: "an image with the tag latest",
			files: &chart.Files{
				Metadata: chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{
					certified: "rancher",
				},
				Values: map[string]any{
					"image": map[string]any{
						"repository": "rancher/fleet",
						"tag":        "latest",
					},
				},
			},
			version:         goodVersion,
			wantAnnotations: []string{},
			wantHIP15:       []string{`values.yaml image: "tag" must not be "latest"`},
			wantVersion:     []string{},
		},
		{
			name: "version errors only",
			files: &chart.Files{
				Metadata: chart.Metadata{Name: "fleet"},
				Annotations: map[string]string{
					certified: "rancher",
				},
			},
			version:         "110.0.3",
			wantAnnotations: []string{},
			wantHIP15:       []string{},
			wantVersion: []string{
				`version "110.0.3" must look like 1XY.Z.W+upA.B.C, optionally followed by -rancher.N, -rc.N, -beta.N or -alpha.N`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Chart(parseRules(t), tt.files, tt.version)
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
			require.Equal(t, tt.wantVersion, got.Version.Errors)
			require.Equal(t, len(tt.wantVersion) == 0, got.Version.Valid)
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
			got, err := Chart(parseRules(t), files, goodVersion)
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
	got, err := Chart(parseRules(t), files, goodVersion)
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
		"hip15-annotations": {"valid": true, "errors": []},
		"version": {"valid": true, "errors": []}
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
			got, err := Chart(parseRules(t), files, goodVersion)
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
		version     bool
		ignoreHIP15 bool
		wantFails   bool
	}{
		{
			name:        "valid",
			annotations: true,
			hip15:       true,
			version:     true,
			wantFails:   false,
		},
		{
			name:        "valid, HIP-15 ignored",
			annotations: true,
			hip15:       true,
			version:     true,
			ignoreHIP15: true,
			wantFails:   false,
		},
		{
			name:        "annotation errors",
			annotations: false,
			hip15:       true,
			version:     true,
			wantFails:   true,
		},
		{
			name:        "annotation errors, HIP-15 ignored",
			annotations: false,
			hip15:       true,
			version:     true,
			ignoreHIP15: true,
			wantFails:   true,
		},
		{
			name:        "HIP-15 errors",
			annotations: true,
			hip15:       false,
			version:     true,
			wantFails:   true,
		},
		{
			name:        "HIP-15 errors, HIP-15 ignored",
			annotations: true,
			hip15:       false,
			version:     true,
			ignoreHIP15: true,
			wantFails:   false,
		},
		{
			name:        "both, HIP-15 ignored",
			annotations: false,
			hip15:       false,
			version:     true,
			ignoreHIP15: true,
			wantFails:   true,
		},
		{
			name:        "version errors",
			annotations: true,
			hip15:       true,
			version:     false,
			wantFails:   true,
		},
		{
			name:        "version errors, HIP-15 ignored",
			annotations: true,
			hip15:       true,
			version:     false,
			ignoreHIP15: true,
			wantFails:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{
				Valid:            tt.annotations && tt.hip15 && tt.version,
				Annotations:      Section{Valid: tt.annotations},
				HIP15Annotations: Section{Valid: tt.hip15},
				Version:          Section{Valid: tt.version},
			}
			require.Equal(t, tt.wantFails, result.Fails(tt.ignoreHIP15))
		})
	}
}

func TestChartVersion(t *testing.T) {
	const (
		standard = `must look like 1XY.Z.W+upA.B.C, optionally followed by -rancher.N, -rc.N, -beta.N or -alpha.N`
		rancher  = "catalog.cattle.io/rancher-version"
	)

	tests := []struct {
		name        string
		chart       string
		version     string
		annotations map[string]string
		wantErrors  []string
	}{
		{
			name:        "a release for the Rancher version of the annotation",
			chart:       "fleet",
			version:     "111.0.3+up0.16.3",
			annotations: map[string]string{rancher: ">= 2.16.0-0 < 2.17.0-0"},
			wantErrors:  []string{},
		},
		{
			name:        "a prerelease",
			chart:       "fleet",
			version:     "112.0.3+up0.16.3-rc.1",
			annotations: map[string]string{rancher: ">= 2.17.0-0 < 2.18.0-0"},
			wantErrors:  []string{},
		},
		{
			name:        "the major of another Rancher version",
			chart:       "fleet",
			version:     "112.0.3+up0.16.3",
			annotations: map[string]string{rancher: ">= 2.16.0-0 < 2.17.0-0"},
			wantErrors: []string{
				`version "112.0.3+up0.16.3" has major 112, but charts for Rancher 2.16 have major 111`,
			},
		},
		{
			name:        "a Rancher version without a major in the rules",
			chart:       "fleet",
			version:     "111.0.3+up0.16.3",
			annotations: map[string]string{rancher: ">= 2.20.0-0 < 2.21.0-0"},
			wantErrors: []string{
				"no chart major for Rancher 2.20 in the rules file",
			},
		},
		{
			name:        "a bad form is not checked against the annotation",
			chart:       "fleet",
			version:     "112.0.3",
			annotations: map[string]string{rancher: ">= 2.16.0-0 < 2.17.0-0"},
			wantErrors:  []string{`version "112.0.3" ` + standard},
		},
		{
			name:        "no rancher-version annotation: the major is not checked",
			chart:       "fleet",
			version:     "999.0.3+up0.16.3",
			annotations: map[string]string{},
			wantErrors:  []string{`version "999.0.3+up0.16.3" ` + standard},
		},
		{
			name:        "no annotation, a good form",
			chart:       "fleet",
			version:     "150.0.3+up0.16.3",
			annotations: map[string]string{},
			wantErrors:  []string{},
		},
		{
			name:        "an annotation without a lower bound: the major is not checked",
			chart:       "fleet",
			version:     "150.0.3+up0.16.3",
			annotations: map[string]string{rancher: "< 2.17.0-0"},
			wantErrors:  []string{},
		},
		{
			name:        "a CRD is checked for its form only",
			chart:       "fleet-crd",
			version:     "150.0.3+up0.16.3",
			annotations: map[string]string{rancher: ">= 2.16.0-0 < 2.17.0-0"},
			wantErrors:  []string{},
		},
		{
			name:        "a CRD with a bad form",
			chart:       "fleet-crd",
			version:     "110.0.3",
			annotations: map[string]string{},
			wantErrors:  []string{`version "110.0.3" ` + standard},
		},
		{
			name:        "a plain chart",
			chart:       "system-upgrade-controller",
			version:     "111.0.0",
			annotations: map[string]string{rancher: ">= 2.16.0-0 < 2.17.0-0"},
			wantErrors:  []string{},
		},
		{
			name:        "a plain chart with a +up part",
			chart:       "system-upgrade-controller",
			version:     "111.0.0+up0.1.2",
			annotations: map[string]string{rancher: ">= 2.16.0-0 < 2.17.0-0"},
			wantErrors:  []string{`version "111.0.0+up0.1.2" must look like 1XY.Z.W`},
		},
		{
			name:        "a plain chart with the wrong major",
			chart:       "system-upgrade-controller",
			version:     "110.0.0",
			annotations: map[string]string{rancher: ">= 2.16.0-0 < 2.17.0-0"},
			wantErrors: []string{
				`version "110.0.0" has major 110, but charts for Rancher 2.16 have major 111`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := &chart.Files{
				Metadata:    chart.Metadata{Name: tt.chart},
				Annotations: tt.annotations,
			}

			got, err := Chart(parseRules(t), files, tt.version)

			require.NoError(t, err)
			require.Equal(t, tt.wantErrors, got.Version.Errors)
			require.Equal(t, len(tt.wantErrors) == 0, got.Version.Valid)
		})
	}
}

// TestChartValidatesTheGivenVersion makes sure the version to validate is the
// one the caller gives, not the one in Chart.yaml.
func TestChartValidatesTheGivenVersion(t *testing.T) {
	files := &chart.Files{
		Metadata:    chart.Metadata{Name: "fleet", Version: "not-a-version"},
		Annotations: map[string]string{"catalog.cattle.io/certified": "rancher"},
	}

	got, err := Chart(parseRules(t), files, goodVersion)

	require.NoError(t, err)
	require.True(t, got.Version.Valid)
	require.Equal(t, "not-a-version", got.Metadata.Version)
}
