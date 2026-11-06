package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/rules"
)

func TestValidate(t *testing.T) {
	required := []string{"release-name", "certified"}
	main := rules.Chart{Name: "fleet", Kind: rules.KindMain, ReleaseName: "fleet", Required: required}
	crd := rules.Chart{Name: "fleet-crd", Kind: rules.KindCRD, ReleaseName: "fleet-crd", Required: required}
	extra := rules.Chart{Name: "fleet-agent", Kind: rules.KindExtra, ReleaseName: "fleet-agent", Required: []string{"certified"}}

	wrongName := map[string]string{
		"catalog.cattle.io/release-name": "other",
		"catalog.cattle.io/certified":    "rancher",
	}

	tests := []struct {
		name        string
		chart       rules.Chart
		annotations map[string]string
		want        []string
	}{
		{name: "main valid", chart: main, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet",
			"catalog.cattle.io/certified":    "rancher",
		}},
		{name: "main missing", chart: main, annotations: map[string]string{"catalog.cattle.io/release-name": "fleet"}, want: []string{"missing annotation catalog.cattle.io/certified"}},
		{name: "main wrong release-name", chart: main, annotations: wrongName, want: []string{`catalog.cattle.io/release-name is "other", want "fleet"`}},

		{name: "crd valid", chart: crd, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet-crd",
			"catalog.cattle.io/certified":    "rancher",
		}},
		{name: "crd missing", chart: crd, annotations: map[string]string{"catalog.cattle.io/release-name": "fleet-crd"}, want: []string{"missing annotation catalog.cattle.io/certified"}},
		{name: "crd wrong release-name", chart: crd, annotations: wrongName, want: []string{`catalog.cattle.io/release-name is "other", want "fleet-crd"`}},

		{name: "extra valid", chart: extra, annotations: map[string]string{"catalog.cattle.io/certified": "rancher"}},
		{name: "extra missing", chart: extra, annotations: nil, want: []string{"missing annotation catalog.cattle.io/certified"}},
		{name: "extra ignores release-name", chart: extra, annotations: wrongName},

		{name: "main bad kube-version", chart: main, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet",
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/kube-version": "< 1.37.0-0",
		}, want: []string{`catalog.cattle.io/kube-version "< 1.37.0-0": must look like ">= 1.34.0-0 < 1.37.0-0"`}},
		{name: "extra bad kube-version", chart: extra, annotations: map[string]string{
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/kube-version": "latest",
		}, want: []string{`catalog.cattle.io/kube-version "latest": must look like ">= 1.34.0-0 < 1.37.0-0"`}},
		{name: "main bad rancher-version", chart: main, annotations: map[string]string{
			"catalog.cattle.io/release-name":    "fleet",
			"catalog.cattle.io/certified":       "rancher",
			"catalog.cattle.io/rancher-version": ">= 2.11.0-0",
		}, want: []string{`catalog.cattle.io/rancher-version ">= 2.11.0-0": must look like ">= 2.15.0-0 < 2.16.0-0"`}},
		{name: "extra bad rancher-version", chart: extra, annotations: map[string]string{
			"catalog.cattle.io/certified":       "rancher",
			"catalog.cattle.io/rancher-version": "latest",
		}, want: []string{`catalog.cattle.io/rancher-version "latest": must look like ">= 2.15.0-0 < 2.16.0-0"`}},
		{name: "main bad certified", chart: main, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet",
			"catalog.cattle.io/certified":    "partner",
		}, want: []string{`catalog.cattle.io/certified "partner": must be "rancher"`}},
		{name: "crd bad certified", chart: crd, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet-crd",
			"catalog.cattle.io/certified":    "partner",
		}, want: []string{`catalog.cattle.io/certified "partner": must be "rancher"`}},
		{name: "extra bad certified", chart: extra, annotations: map[string]string{
			"catalog.cattle.io/certified": "partner",
		}, want: []string{`catalog.cattle.io/certified "partner": must be "rancher"`}},
		{name: "main bad permits-os", chart: main, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet",
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/permits-os":   "windows",
		}, want: []string{`catalog.cattle.io/permits-os "windows": must include "linux"`}},
		{name: "extra bad permits-os", chart: extra, annotations: map[string]string{
			"catalog.cattle.io/certified":  "rancher",
			"catalog.cattle.io/permits-os": "linux,darwin",
		}, want: []string{`catalog.cattle.io/permits-os "linux,darwin": must list "linux", and optionally "windows", separated by commas`}},
		{name: "crd does not check permits-os", chart: crd, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet-crd",
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/hidden":       "true",
			"catalog.cattle.io/permits-os":   "windows",
		}},
		{name: "crd bad hidden", chart: crd, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet-crd",
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/hidden":       "yes",
		}, want: []string{`catalog.cattle.io/hidden "yes": must be "true"`}},
		{name: "crd hidden false", chart: crd, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet-crd",
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/hidden":       "false",
		}, want: []string{`catalog.cattle.io/hidden "false": must be "true"`}},
		{name: "main does not check hidden", chart: main, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet",
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/hidden":       "yes",
		}},
		{name: "extra does not check hidden", chart: extra, annotations: map[string]string{
			"catalog.cattle.io/certified": "rancher",
			"catalog.cattle.io/hidden":    "yes",
		}},
		{name: "crd does not check rancher-version", chart: crd, annotations: map[string]string{
			"catalog.cattle.io/release-name":    "fleet-crd",
			"catalog.cattle.io/certified":       "rancher",
			"catalog.cattle.io/rancher-version": "latest",
		}},
		{name: "crd does not check kube-version", chart: crd, annotations: map[string]string{
			"catalog.cattle.io/release-name": "fleet-crd",
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/kube-version": "latest",
		}},

		{name: "unknown kind", chart: rules.Chart{Name: "x", Kind: "other"}, annotations: nil, want: []string{`chart x has unknown kind "other"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Validate(tt.chart, tt.annotations))
		})
	}
}

func TestCheckMissing(t *testing.T) {
	chart := rules.Chart{Required: []string{"release-name", "certified"}}

	tests := []struct {
		name        string
		annotations map[string]string
		want        []string
	}{
		{name: "none present", annotations: map[string]string{}, want: []string{"missing annotation catalog.cattle.io/release-name", "missing annotation catalog.cattle.io/certified"}},
		{name: "nil annotations", annotations: nil, want: []string{"missing annotation catalog.cattle.io/release-name", "missing annotation catalog.cattle.io/certified"}},
		{
			name: "empty and blank values count as missing",
			annotations: map[string]string{
				"catalog.cattle.io/release-name": "",
				"catalog.cattle.io/certified":    "  ",
			},
			want: []string{"missing annotation catalog.cattle.io/release-name", "missing annotation catalog.cattle.io/certified"},
		},
		{
			name: "same name under another prefix does not count",
			annotations: map[string]string{
				"catalog.rancher.io/release-name": "fleet",
				"catalog.cattle.io/certified":     "rancher",
			},
			want: []string{"missing annotation catalog.cattle.io/release-name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, checkMissing(chart, tt.annotations))
		})
	}

	require.Empty(t, checkMissing(rules.Chart{}, map[string]string{"catalog.cattle.io/hidden": "true"}))
}

func TestCheckReleaseName(t *testing.T) {
	chart := rules.Chart{Name: "elemental", ReleaseName: "elemental-operator"}

	tests := []struct {
		name        string
		annotations map[string]string
		want        []string
	}{
		{name: "matches", annotations: map[string]string{"catalog.cattle.io/release-name": "elemental-operator"}},
		{name: "differs", annotations: map[string]string{"catalog.cattle.io/release-name": "elemental"}, want: []string{`catalog.cattle.io/release-name is "elemental", want "elemental-operator"`}},
		{name: "differs in case", annotations: map[string]string{"catalog.cattle.io/release-name": "Elemental-Operator"}, want: []string{`catalog.cattle.io/release-name is "Elemental-Operator", want "elemental-operator"`}},
		{name: "surrounding space is ignored", annotations: map[string]string{"catalog.cattle.io/release-name": "  elemental-operator "}},
		{name: "differs, reported without the surrounding space", annotations: map[string]string{"catalog.cattle.io/release-name": " elemental "}, want: []string{`catalog.cattle.io/release-name is "elemental", want "elemental-operator"`}},
		{name: "absent is left to checkMissing", annotations: map[string]string{}},
		{name: "nil annotations", annotations: nil},
		{name: "blank is left to checkMissing", annotations: map[string]string{"catalog.cattle.io/release-name": " "}},
		{name: "other prefix is ignored", annotations: map[string]string{"catalog.rancher.io/release-name": "other"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, checkReleaseName(chart, tt.annotations))
		})
	}
}
