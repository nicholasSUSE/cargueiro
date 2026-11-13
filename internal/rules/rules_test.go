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

// registriesAndVersions are the sections a file needs besides the charts.
const registriesAndVersions = `
registries:
  rancher-charts: {charts-staging: s.io/a, charts-production: p.io/a}
  prime-charts: {charts-staging: s.io/b, charts-production: p.io/b}
  images: {staging: s.io/c, production: p.io/c}
versions:
  prereleases: [rc]
  majors: {"2.16": 111}
`

func TestParseRegistries(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)

	want := Registries{
		RancherCharts: ChartRegistries{
			Staging:    "stgregistry.suse.com/rancher/catalog",
			Production: "registry.suse.com/rancher/catalog",
		},
		PrimeCharts: ChartRegistries{
			Staging:    "stgregistry.suse.com/rancher/charts",
			Production: "registry.suse.com/rancher/charts",
		},
		Images: ImageRegistries{
			Staging:    "stgregistry.suse.com/rancher",
			Production: "registry.suse.com/rancher",
		},
	}
	require.Equal(t, want, cfg.Registries())
}

func TestParseVersions(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)

	want := Versions{
		Prereleases: []string{"rancher", "rc", "beta", "alpha"},
		Majors:      map[string]int{"2.16": 111, "2.17": 112},
	}
	require.Equal(t, want, cfg.Versions())

	// The caller gets a copy, not the policy itself.
	got := cfg.Versions()
	got.Prereleases[0] = "changed"
	got.Majors["2.16"] = 1
	require.Equal(t, want, cfg.Versions())
}

func TestParsePlainVersion(t *testing.T) {
	const charts = `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts, version: plain}
  - family: b
    charts:
      - {name: b, kind: main, edition: rancher-charts}
`
	cfg, err := Parse([]byte(charts + registriesAndVersions))
	require.NoError(t, err)

	a, err := cfg.Find("a")
	require.NoError(t, err)
	require.True(t, a.PlainVersion)

	b, err := cfg.Find("b")
	require.NoError(t, err)
	require.False(t, b.PlainVersion)
}

func TestParseRejectsVersions(t *testing.T) {
	const charts = `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
`
	const registries = `
registries:
  rancher-charts: {charts-staging: s.io/a, charts-production: p.io/a}
  prime-charts: {charts-staging: s.io/b, charts-production: p.io/b}
  images: {staging: s.io/c, production: p.io/c}
`

	tests := []struct {
		name     string
		versions string
		wantErr  string
	}{
		{
			name:     "no versions section",
			versions: "",
			wantErr:  "versions: no prereleases",
		},
		{
			name:     "no majors",
			versions: "versions: {prereleases: [rc]}",
			wantErr:  "versions: no majors",
		},
		{
			name:     "an uppercase prerelease",
			versions: `versions: {prereleases: [RC], majors: {"2.16": 111}}`,
			wantErr:  `prerelease "RC" must be lowercase letters`,
		},
		{
			name:     "a prerelease with a dot",
			versions: `versions: {prereleases: ["rc."], majors: {"2.16": 111}}`,
			wantErr:  `prerelease "rc." must be lowercase letters`,
		},
		{
			name:     "a prerelease listed twice",
			versions: `versions: {prereleases: [rc, rc], majors: {"2.16": 111}}`,
			wantErr:  `prerelease "rc" is listed twice`,
		},
		{
			name:     "a Rancher version that is not 2.N",
			versions: `versions: {prereleases: [rc], majors: {"3.1": 111}}`,
			wantErr:  `Rancher version "3.1" must look like 2.16`,
		},
		{
			name:     "a major below 100",
			versions: `versions: {prereleases: [rc], majors: {"2.16": 99}}`,
			wantErr:  "major 99 of Rancher 2.16 must be between 100 and 199",
		},
		{
			name:     "a major above 199",
			versions: `versions: {prereleases: [rc], majors: {"2.16": 200}}`,
			wantErr:  "major 200 of Rancher 2.16 must be between 100 and 199",
		},
		{
			name:     "a major used twice",
			versions: `versions: {prereleases: [rc], majors: {"2.16": 111, "2.17": 111}}`,
			wantErr:  "major 111 is used by Rancher 2.16 and 2.17",
		},
		{
			name:     "an unknown key",
			versions: `versions: {prereleases: [rc], majors: {"2.16": 111}, other: x}`,
			wantErr:  "field other not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse([]byte(charts + registries + tt.versions))
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, cfg)
		})
	}
}

func TestIconExempt(t *testing.T) {
	const in = `
kinds: {main: {}, crd: {}, extra: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
      - {name: a-crd, kind: crd, edition: rancher-charts}
      - {name: a-extra, kind: extra, edition: rancher-charts}
  - family: skipped
    charts:
      - {name: skipped, kind: main, edition: rancher-charts, icon: skip}
`

	tests := []struct {
		name         string
		chart        string
		wantSkipIcon bool
		wantExempt   bool
	}{
		{
			name:         "a main chart needs an icon",
			chart:        "a",
			wantSkipIcon: false,
			wantExempt:   false,
		},
		{
			name:         "a main chart marked icon: skip",
			chart:        "skipped",
			wantSkipIcon: true,
			wantExempt:   true,
		},
		{
			name:         "a crd chart is exempt by its kind",
			chart:        "a-crd",
			wantSkipIcon: false,
			wantExempt:   true,
		},
		{
			name:         "an extra chart is exempt by its kind",
			chart:        "a-extra",
			wantSkipIcon: false,
			wantExempt:   true,
		},
	}

	cfg, err := Parse([]byte(in + registriesAndVersions))
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.Find(tt.chart)
			require.NoError(t, err)
			require.Equal(t, tt.wantSkipIcon, got.SkipIcon)
			require.Equal(t, tt.wantExempt, got.IconExempt())
		})
	}
}

func TestParseRejectsAnIcon(t *testing.T) {
	tests := []struct {
		name    string
		chart   string
		wantErr string
	}{
		{
			name:    "icon: skip on a crd chart",
			chart:   "{name: a, kind: crd, edition: rancher-charts, icon: skip}",
			wantErr: `chart "a": icon: skip is for a main chart; a crd or extra chart is exempt already`,
		},
		{
			name:    "icon: skip on an extra chart",
			chart:   "{name: a, kind: extra, edition: rancher-charts, icon: skip}",
			wantErr: `chart "a": icon: skip is for a main chart; a crd or extra chart is exempt already`,
		},
		{
			name:    "an unknown icon value",
			chart:   "{name: a, kind: main, edition: rancher-charts, icon: none}",
			wantErr: `chart "a" has unknown icon "none"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := "\nkinds: {main: {}, crd: {}, extra: {}}\nfamilies:\n  - family: a\n    charts:\n      - " + tt.chart + "\n"

			cfg, err := Parse([]byte(in))
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, cfg)
		})
	}
}

func TestParseReadsTheUpstream(t *testing.T) {
	const in = `
kinds: {main: {}, crd: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts, upstream: https://github.com/rancher/webhook}
      - {name: a-crd, kind: crd, edition: rancher-charts}
  - family: b
    charts:
      - {name: b, kind: main, edition: rancher-charts}
`
	cfg, err := Parse([]byte(in + registriesAndVersions))
	require.NoError(t, err)

	got, err := cfg.Find("a")
	require.NoError(t, err)
	require.Equal(t, "https://github.com/rancher/webhook", got.Upstream)
	require.Equal(t, "https://github.com/rancher/webhook", mustFamily(t, cfg, "a").Main().Upstream)

	for _, name := range []string{"a-crd", "b"} {
		got, err := cfg.Find(name)
		require.NoError(t, err)
		require.Empty(t, got.Upstream, "the upstream is optional: %s", name)
	}
}

func TestMainCharts(t *testing.T) {
	const in = `
kinds: {main: {}, crd: {}, extra: {}}
families:
  - family: zeta
    charts:
      - {name: zeta-crd, kind: crd, edition: rancher-charts}
      - {name: zeta, kind: main, edition: rancher-charts, upstream: https://github.com/rancher/zeta}
  - family: alpha
    charts:
      - {name: alpha, kind: main, edition: prime-charts}
      - {name: alpha-extra, kind: extra, edition: prime-charts}
`
	cfg, err := Parse([]byte(in + registriesAndVersions))
	require.NoError(t, err)

	got := cfg.MainCharts()

	require.Len(t, got, 2)
	require.Equal(t, "alpha", got[0].Name)
	require.Empty(t, got[0].Upstream)
	require.Equal(t, "zeta", got[1].Name)
	require.Equal(t, "https://github.com/rancher/zeta", got[1].Upstream)
}

func mustFamily(t *testing.T, cfg *Config, name string) Family {
	t.Helper()

	family, err := cfg.Family(name)
	require.NoError(t, err)
	return family
}

func TestParseRejectsAnUpstream(t *testing.T) {
	const form = "which must look like https://github.com/<organization>/<repository>"
	tests := []struct {
		name    string
		chart   string
		wantErr string
	}{
		{
			name:    "on a crd chart",
			chart:   "{name: a, kind: crd, edition: rancher-charts, upstream: https://github.com/rancher/webhook}",
			wantErr: `chart "a": upstream is for a main chart; a crd or extra chart is released from the repository of its main chart`,
		},
		{
			name:    "on an extra chart",
			chart:   "{name: a, kind: extra, edition: rancher-charts, upstream: https://github.com/rancher/webhook}",
			wantErr: `chart "a": upstream is for a main chart`,
		},
		{
			name:    "without the scheme",
			chart:   "{name: a, kind: main, edition: rancher-charts, upstream: github.com/rancher/webhook}",
			wantErr: form,
		},
		{
			name:    "another host",
			chart:   "{name: a, kind: main, edition: rancher-charts, upstream: https://gitlab.com/rancher/webhook}",
			wantErr: form,
		},
		{
			name:    "a trailing slash",
			chart:   "{name: a, kind: main, edition: rancher-charts, upstream: https://github.com/rancher/webhook/}",
			wantErr: form,
		},
		{
			name:    "a .git suffix",
			chart:   "{name: a, kind: main, edition: rancher-charts, upstream: https://github.com/rancher/webhook.git}",
			wantErr: form,
		},
		{
			name:    "a path under the repository",
			chart:   "{name: a, kind: main, edition: rancher-charts, upstream: https://github.com/rancher/webhook/tree/main}",
			wantErr: form,
		},
		{
			name:    "only the organization",
			chart:   "{name: a, kind: main, edition: rancher-charts, upstream: https://github.com/rancher}",
			wantErr: form,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := "\nkinds: {main: {}, crd: {}, extra: {}}\nfamilies:\n  - family: a\n    charts:\n      - " + tt.chart + "\n"

			cfg, err := Parse([]byte(in))
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, cfg)
		})
	}
}

func TestParseRejectsAnUnknownChartVersion(t *testing.T) {
	const in = `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts, version: other}
`
	cfg, err := Parse([]byte(in))
	require.ErrorContains(t, err, `chart "a" has unknown version "other"`)
	require.Nil(t, cfg)
}

func TestRegistriesRepositories(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)
	registries := cfg.Registries()

	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "rancher-charts",
			got:  registries.ChartRepositories(EditionRancherCharts),
			want: []string{"stgregistry.suse.com/rancher/catalog", "registry.suse.com/rancher/catalog"},
		},
		{
			name: "prime-charts",
			got:  registries.ChartRepositories(EditionPrimeCharts),
			want: []string{"stgregistry.suse.com/rancher/charts", "registry.suse.com/rancher/charts"},
		},
		{
			name: "unknown edition",
			got:  registries.ChartRepositories("other"),
			want: nil,
		},
		{
			name: "images",
			got:  registries.ImageRepositories(),
			want: []string{"stgregistry.suse.com/rancher", "registry.suse.com/rancher"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.got)
		})
	}
}

func TestStagingRepository(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)
	elemental, err := cfg.Find("elemental")
	require.NoError(t, err)

	tests := []struct {
		name     string
		chart    Chart
		registry string
		want     string
		wantErr  string
	}{
		{
			name:  "the staging registry of the edition",
			chart: elemental,
			want:  "stgregistry.suse.com/rancher/catalog/elemental",
		},
		{
			name:     "a registry given in its place",
			chart:    elemental,
			registry: "127.0.0.1:5050/e2e",
			want:     "127.0.0.1:5050/e2e/elemental",
		},
		{
			name:     "a registry given with a trailing slash",
			chart:    elemental,
			registry: "127.0.0.1:5050/e2e/",
			want:     "127.0.0.1:5050/e2e/elemental",
		},
		{
			name:    "an edition with no registry",
			chart:   Chart{Name: "other", Edition: "other"},
			wantErr: `the rules file has no registry for the edition "other"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.StagingRepository(tt.chart, tt.registry)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEditions(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)

	require.Equal(t, map[string]string{
		"elemental":     EditionRancherCharts,
		"elemental-crd": EditionRancherCharts,
	}, cfg.Editions())
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
			name: "family without a main chart",
			in: `
kinds: {main: {}, crd: {}}
families:
  - family: a
    charts:
      - {name: a-crd, kind: crd, edition: rancher-charts}
`,
			wantErr: `family "a" must have exactly one main chart, it has 0`,
		},
		{
			name: "family with two main charts",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
      - {name: b, kind: main, edition: rancher-charts}
`,
			wantErr: `family "a" must have exactly one main chart, it has 2`,
		},
		{
			name: "family listed twice",
			in: `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
  - family: a
    charts:
      - {name: b, kind: main, edition: rancher-charts}
`,
			wantErr: `family "a" is listed twice`,
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

func TestParseRejectsRegistries(t *testing.T) {
	const charts = `
kinds: {main: {}}
families:
  - family: a
    charts:
      - {name: a, kind: main, edition: rancher-charts}
`

	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{
			name:    "no registries section",
			in:      charts,
			wantErr: `registry "rancher-charts.charts-staging": missing`,
		},
		{
			name: "a registry is missing",
			in: charts + `
registries:
  rancher-charts: {charts-staging: s.io/a, charts-production: p.io/a}
  prime-charts: {charts-staging: s.io/b, charts-production: p.io/b}
  images: {staging: s.io/c}
`,
			wantErr: `registry "images.production": missing`,
		},
		{
			name: "a registry has a scheme",
			in: charts + `
registries:
  rancher-charts: {charts-staging: 'https://s.io/a', charts-production: p.io/a}
  prime-charts: {charts-staging: s.io/b, charts-production: p.io/b}
  images: {staging: s.io/c, production: p.io/c}
`,
			wantErr: `registry "rancher-charts.charts-staging": must not have a scheme`,
		},
		{
			name: "a registry ends in a slash",
			in: charts + `
registries:
  rancher-charts: {charts-staging: s.io/a, charts-production: p.io/a}
  prime-charts: {charts-staging: s.io/b, charts-production: 'p.io/b/'}
  images: {staging: s.io/c, production: p.io/c}
`,
			wantErr: `registry "prime-charts.charts-production": must not end in a slash`,
		},
		{
			name: "an unknown registry key",
			in: charts + `
registries:
  rancher-charts: {charts-staging: s.io/a, charts-production: p.io/a, other: x}
`,
			wantErr: "field other not found",
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

	suc, err := cfg.Find("system-upgrade-controller")
	require.NoError(t, err)
	require.True(t, suc.PlainVersion)
	require.True(t, suc.IconExempt())

	fleet, err := cfg.Find("fleet")
	require.NoError(t, err)
	require.False(t, fleet.PlainVersion)
	require.True(t, fleet.SkipIcon)

	fleetAgent, err := cfg.Find("fleet-agent")
	require.NoError(t, err)
	require.False(t, fleetAgent.SkipIcon, "an extra chart is exempt by its kind")
	require.True(t, fleetAgent.IconExempt())

	monitoring, err := cfg.Find("rancher-monitoring")
	require.NoError(t, err)
	require.False(t, monitoring.IconExempt(), "a chart with an icon to make local")

	require.Equal(t, 111, cfg.Versions().Majors["2.16"])
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
  - family: b
    charts:
      - {name: b, kind: main, edition: rancher-charts}
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

	cfg, err := Parse([]byte(in))
	require.NoError(t, err)
	a, err := cfg.Find("a")
	require.NoError(t, err)
	b, err := cfg.Find("b")
	require.NoError(t, err)

	a.Required[0] = "changed"
	require.Equal(t, []string{"release-name"}, b.Required)
}

func TestFamily(t *testing.T) {
	const in = `
kinds: {main: {}, crd: {}, extra: {}}
families:
  - family: fleet
    charts:
      - {name: fleet-crd, kind: crd, edition: rancher-charts}
      - {name: fleet-agent, kind: extra, edition: rancher-charts}
      - {name: fleet, kind: main, edition: rancher-charts}
  - family: alone
    charts:
      - {name: alone, kind: main, edition: prime-charts}
`
	cfg, err := Parse([]byte(in + registriesAndVersions))
	require.NoError(t, err)

	tests := []struct {
		name        string
		family      string
		wantNames   []string
		wantMain    string
		wantEdition string
	}{
		{
			name:        "the main chart comes first, the others keep their order",
			family:      "fleet",
			wantNames:   []string{"fleet", "fleet-crd", "fleet-agent"},
			wantMain:    "fleet",
			wantEdition: EditionRancherCharts,
		},
		{
			name:        "a family of one chart",
			family:      "alone",
			wantNames:   []string{"alone"},
			wantMain:    "alone",
			wantEdition: EditionPrimeCharts,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.Family(tt.family)
			require.NoError(t, err)
			require.Equal(t, tt.family, got.Name)
			require.Equal(t, tt.wantEdition, got.Edition)
			require.Equal(t, tt.wantNames, got.Names())
			require.Equal(t, tt.wantMain, got.Main().Name)
		})
	}
}

func TestFamilyUnlisted(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)

	_, err = cfg.Family("nope")

	require.EqualError(t, err, `family "nope" is not listed in the rules file (rules.yaml)`)
}

func TestFamilyIsACopy(t *testing.T) {
	cfg, err := Parse([]byte(valid))
	require.NoError(t, err)
	first, err := cfg.Family("elemental")
	require.NoError(t, err)

	first.Charts[0] = Chart{Name: "changed"}

	again, err := cfg.Family("elemental")
	require.NoError(t, err)
	require.Equal(t, []string{"elemental", "elemental-crd"}, again.Names())
}

// Every family of the rules file that cargueiro ships has exactly one main
// chart, first, and one edition.
func TestShippedRulesFamilies(t *testing.T) {
	data, err := os.ReadFile("../../rules.yaml")
	require.NoError(t, err)
	cfg, err := Parse(data)
	require.NoError(t, err)

	fleet, err := cfg.Family("fleet")
	require.NoError(t, err)
	require.Equal(t, []string{"fleet", "fleet-agent", "fleet-crd"}, fleet.Names())
	require.Equal(t, KindMain, fleet.Main().Kind)
}
