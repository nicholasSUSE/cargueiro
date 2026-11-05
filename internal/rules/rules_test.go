package rules

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const valid = `
kinds:
  main:
    required: [release-name, kube-version]
  crd:
    required: [hidden]
families:
  - family: elemental
    charts:
      - { name: elemental, kind: main, release-name: elemental-operator }
      - { name: elemental-crd, kind: crd }
`

func TestParse(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)

	tests := []struct {
		name string
		want Chart
	}{
		{
			name: "elemental",
			want: Chart{
				Name: "elemental", Kind: "main", Family: "elemental",
				ReleaseName: "elemental-operator", Required: []string{"release-name", "kube-version"},
			},
		},
		{
			name: "elemental-crd",
			want: Chart{
				Name: "elemental-crd", Kind: "crd", Family: "elemental",
				ReleaseName: "elemental-crd", Required: []string{"hidden"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cfg.Lookup(tt.name)
			require.True(t, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestChartMissing(t *testing.T) {
	chart := Chart{Required: []string{"release-name", "certified", "kube-version"}}

	tests := []struct {
		name        string
		annotations map[string]string
		want        []string
	}{
		{
			name: "all present",
			annotations: map[string]string{
				"catalog.cattle.io/release-name": "fleet",
				"catalog.cattle.io/certified":    "rancher",
				"catalog.cattle.io/kube-version": ">= 1.34.0-0",
			},
		},
		{
			name:        "none present",
			annotations: map[string]string{},
			want:        []string{"release-name", "certified", "kube-version"},
		},
		{
			name:        "nil annotations",
			annotations: nil,
			want:        []string{"release-name", "certified", "kube-version"},
		},
		{
			name: "one missing",
			annotations: map[string]string{
				"catalog.cattle.io/release-name": "fleet",
				"catalog.cattle.io/kube-version": ">= 1.34.0-0",
			},
			want: []string{"certified"},
		},
		{
			name: "empty and blank values count as missing",
			annotations: map[string]string{
				"catalog.cattle.io/release-name": "",
				"catalog.cattle.io/certified":    "  ",
				"catalog.cattle.io/kube-version": ">= 1.34.0-0",
			},
			want: []string{"release-name", "certified"},
		},
		{
			name: "same name under another prefix does not count",
			annotations: map[string]string{
				"catalog.rancher.io/release-name": "fleet",
				"catalog.cattle.io/certified":     "rancher",
				"catalog.cattle.io/kube-version":  ">= 1.34.0-0",
			},
			want: []string{"release-name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, chart.Missing(tt.annotations))
		})
	}
}

func TestChartMissingNothingRequired(t *testing.T) {
	require.Empty(t, Chart{}.Missing(map[string]string{"catalog.cattle.io/hidden": "true"}))
}

func TestChartReleaseNameMismatch(t *testing.T) {
	chart := Chart{Name: "elemental", ReleaseName: "elemental-operator"}

	tests := []struct {
		name        string
		annotations map[string]string
		wantGot     string
		wantBad     bool
	}{
		{
			name:        "matches",
			annotations: map[string]string{"catalog.cattle.io/release-name": "elemental-operator"},
			wantGot:     "elemental-operator",
		},
		{
			name:        "differs",
			annotations: map[string]string{"catalog.cattle.io/release-name": "elemental"},
			wantGot:     "elemental",
			wantBad:     true,
		},
		{
			name:        "differs in case",
			annotations: map[string]string{"catalog.cattle.io/release-name": "Elemental-Operator"},
			wantGot:     "Elemental-Operator",
			wantBad:     true,
		},
		{
			name:        "differs by surrounding space",
			annotations: map[string]string{"catalog.cattle.io/release-name": " elemental-operator"},
			wantGot:     " elemental-operator",
			wantBad:     true,
		},
		{name: "absent is left to Missing", annotations: map[string]string{}},
		{name: "nil annotations", annotations: nil},
		{
			name:        "empty is left to Missing",
			annotations: map[string]string{"catalog.cattle.io/release-name": " "},
		},
		{
			name:        "other prefix is ignored",
			annotations: map[string]string{"catalog.rancher.io/release-name": "other"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, bad := chart.ReleaseNameMismatch(tt.annotations)
			require.Equal(t, tt.wantGot, got)
			require.Equal(t, tt.wantBad, bad)
		})
	}
}

func TestLookupUnlistedChart(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)

	_, ok := cfg.Lookup("epinio")
	require.False(t, ok)
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "empty file", in: "", wantErr: "empty file"},
		{name: "not yaml", in: "kinds: [", wantErr: "parse rules"},
		{name: "unknown top-level key", in: "kind: {}\n", wantErr: "field kind not found"},
		{
			name:    "unknown chart key",
			in:      "kinds: {main: {}}\nfamilies:\n  - family: a\n    charts:\n      - {name: a, kind: main, colour: red}\n",
			wantErr: "field colour not found",
		},
		{name: "no charts at all", in: "kinds: {main: {}}\n", wantErr: "no charts listed"},
		{
			name:    "family without a name",
			in:      "kinds: {main: {}}\nfamilies:\n  - charts:\n      - {name: a, kind: main}\n",
			wantErr: "a family has no name",
		},
		{
			name:    "family without charts",
			in:      "kinds: {main: {}}\nfamilies:\n  - family: a\n",
			wantErr: `family "a" has no charts`,
		},
		{
			name:    "chart without a name",
			in:      "kinds: {main: {}}\nfamilies:\n  - family: a\n    charts:\n      - {kind: main}\n",
			wantErr: `family "a" has a chart with no name`,
		},
		{
			name:    "undefined kind",
			in:      "kinds: {main: {}}\nfamilies:\n  - family: a\n    charts:\n      - {name: a, kind: other}\n",
			wantErr: `chart "a" has undefined kind "other"`,
		},
		{
			name:    "chart without a kind",
			in:      "kinds: {main: {}}\nfamilies:\n  - family: a\n    charts:\n      - {name: a}\n",
			wantErr: `chart "a" has undefined kind ""`,
		},
		{
			name:    "chart listed twice in one family",
			in:      "kinds: {main: {}}\nfamilies:\n  - family: a\n    charts:\n      - {name: a, kind: main}\n      - {name: a, kind: main}\n",
			wantErr: `chart "a" is listed twice`,
		},
		{
			name:    "chart listed in two families",
			in:      "kinds: {main: {}}\nfamilies:\n  - family: a\n    charts:\n      - {name: x, kind: main}\n  - family: b\n    charts:\n      - {name: x, kind: main}\n",
			wantErr: `chart "x" is listed twice`,
		},
		{
			name:    "empty annotation name",
			in:      "kinds: {main: {required: ['']}}\nfamilies:\n  - family: a\n    charts:\n      - {name: a, kind: main}\n",
			wantErr: `kind "main": empty annotation name`,
		},
		{
			name:    "annotation required twice",
			in:      "kinds: {main: {required: [hidden, hidden]}}\nfamilies:\n  - family: a\n    charts:\n      - {name: a, kind: main}\n",
			wantErr: `kind "main": annotation "hidden" is required twice`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse([]byte(tt.in))
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, cfg)
		})
	}
}

// TestRepositoryConfig keeps the rules file at the repository root valid.
func TestRepositoryConfig(t *testing.T) {
	data, err := os.ReadFile("../../rules.yaml")
	require.NoError(t, err)

	cfg, err := Parse(data)
	require.NoError(t, err)

	chart, ok := cfg.Lookup("rancher-backup-crd")
	require.True(t, ok)
	require.Equal(t, "crd", chart.Kind)
	require.Equal(t, "rancher-backup", chart.Family)
}
