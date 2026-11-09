package rules

import (
	"fmt"
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
      - { name: elemental, kind: main, release-name: elemental-operator, edition: rancher-charts }
      - { name: elemental-crd, kind: crd, edition: rancher-charts }
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
				Name:        "elemental",
				Kind:        "main",
				Family:      "elemental",
				Edition:     "rancher-charts",
				ReleaseName: "elemental-operator",
				Required:    []string{"release-name", "kube-version"},
			},
		},
		{
			name: "elemental-crd",
			want: Chart{
				Name:        "elemental-crd",
				Kind:        "crd",
				Family:      "elemental",
				Edition:     "rancher-charts",
				ReleaseName: "elemental-crd",
				Required:    []string{"hidden"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.Find(tt.name)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFindUnlistedChart(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)

	tests := []struct {
		name  string
		chart string
	}{
		{
			name:  "a chart that is not in the file",
			chart: "epinio",
		},
		{
			name:  "different case",
			chart: "Elemental",
		},
		{
			name:  "empty name",
			chart: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.Find(tt.chart)
			require.ErrorContains(t, err, fmt.Sprintf("chart %q is not listed", tt.chart))
			require.Equal(t, Chart{}, got)
		})
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{
			name:    "empty file",
			in:      "",
			wantErr: "empty file",
		},
		{
			name:    "not yaml",
			in:      "kinds: [",
			wantErr: "did not find expected node content",
		},
		{
			name: "unknown top-level key",
			in: `
kind: {}
`,
			wantErr: "field kind not found",
		},
		{
			name: "unknown chart key",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, colour: red, edition: rancher-charts}
`,
			wantErr: "field colour not found",
		},
		{
			name: "unknown kind",
			in: `
kinds: {other: {}}
`,
			wantErr: `unknown kind "other"`,
		},
		{
			name: "no charts at all",
			in: `
kinds: {main: {}}
`,
			wantErr: "no charts listed",
		},
		{
			name: "family without a name",
			in: `
kinds: {main: {}}
families:
  - charts:
      - {name: a, kind: main, edition: rancher-charts}
`,
			wantErr: "a family has no name",
		},
		{
			name: "family without charts",
			in: `
kinds: {main: {}}
families:
  - family: a
`,
			wantErr: `family "a" has no charts`,
		},
		{
			name: "chart without a name",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {kind: main, edition: rancher-charts}
`,
			wantErr: `family "a" has a chart with no name`,
		},
		{
			name: "undefined kind",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: other, edition: rancher-charts}
`,
			wantErr: `chart "a" has undefined kind "other"`,
		},
		{
			name: "chart without a kind",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a}
`,
			wantErr: `chart "a" has no kind`,
		},
		{
			name: "chart without an edition",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main}
`,
			wantErr: `chart "a" has no edition`,
		},
		{
			name: "unknown edition",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: other}
`,
			wantErr: `chart "a" has unknown edition "other"`,
		},
		{
			name: "family mixing editions",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
      - {name: b, kind: main, edition: prime-charts}
`,
			wantErr: `family "a" mixes editions "rancher-charts" and "prime-charts"`,
		},
		{
			name: "chart listed twice in one family",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
      - {name: a, kind: main, edition: rancher-charts}
`,
			wantErr: `chart "a" is listed twice`,
		},
		{
			name: "chart listed in two families",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: x, kind: main, edition: rancher-charts}
  - family: b
    charts:
      - {name: x, kind: main, edition: rancher-charts}
`,
			wantErr: `chart "x" is listed twice`,
		},
		{
			name: "empty annotation name",
			in: `
kinds: {main: {required: ['']}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
`,
			wantErr: `kind "main": empty annotation name`,
		},
		{
			name: "annotation required twice",
			in: `
kinds: {main: {required: [hidden, hidden]}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
`,
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

	chart, err := cfg.Find("rancher-backup-crd")
	require.NoError(t, err)
	require.Equal(t, "crd", chart.Kind)
	require.Equal(t, "rancher-backup", chart.Family)
	require.Equal(t, EditionRancherCharts, chart.Edition)

	agent, err := cfg.Find("rancher-ai-agent")
	require.NoError(t, err)
	require.Equal(t, EditionPrimeCharts, agent.Edition)
	require.Equal(t, "cattle-ai-agent-system", agent.ReleaseName)
}

// TestParseReportsTheSameKindEveryTime keeps the error stable when several
// kinds are wrong: Go ranges over a map in random order.
func TestParseReportsTheSameKindEveryTime(t *testing.T) {
	const in = `
kinds:
  zeta: {}
  alpha: {}
  middle: {}
`

	for range 20 {
		cfg, err := Parse([]byte(in))
		require.ErrorContains(t, err, `unknown kind "alpha"`)
		require.Nil(t, cfg)
	}
}

// TestChartsOfAKindDoNotShareRequired makes sure that a caller who changes the
// Required list of one chart does not change it for the others.
func TestChartsOfAKindDoNotShareRequired(t *testing.T) {
	const in = `
kinds:
  main:
    required: [release-name]
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
      - {name: b, kind: main, edition: rancher-charts}
`

	cfg, err := Parse([]byte(in))
	require.NoError(t, err)
	a, err := cfg.Find("a")
	require.NoError(t, err)
	b, err := cfg.Find("b")
	require.NoError(t, err)

	a.Required[0] = "changed"
	require.Equal(t, []string{"release-name"}, b.Required)
}
