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
