package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

const upstreamRules = `
kinds:
  main: {}
  crd: {}
families:
  - family: zeta
    charts:
      - { name: zeta, kind: main, edition: rancher-charts }
      - { name: zeta-crd, kind: crd, edition: rancher-charts }
  - family: fleet
    charts:
      - { name: fleet, kind: main, edition: rancher-charts, upstream: https://github.com/rancher/fleet }
  - family: rancher-backup
    charts:
      - { name: rancher-backup, kind: main, edition: rancher-charts, upstream: https://github.com/rancher/backup-restore-operator }
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
  prereleases: [rc]
  majors: {"2.16": 111}
`

func runUpstreamRepos(t *testing.T, rulesFile string, args ...string) (stdout string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(rulesFile),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{})
	require.NoError(t, err)
	root.SetArgs(append([]string{"upstream-repos"}, args...))

	err = root.Execute()
	return out.String(), err
}

func TestUpstreamReposListsTheMainCharts(t *testing.T) {
	stdout, err := runUpstreamRepos(t, upstreamRules)

	require.NoError(t, err)
	require.Equal(t, ""+
		"fleet           https://github.com/rancher/fleet\n"+
		"rancher-backup  https://github.com/rancher/backup-restore-operator\n"+
		"zeta            needs an upstream\n", stdout)
	require.NotContains(t, stdout, "zeta-crd", "a crd chart has no line")
}

func TestUpstreamReposTakesNoArguments(t *testing.T) {
	stdout, err := runUpstreamRepos(t, upstreamRules, "fleet")

	require.ErrorContains(t, err, "unknown command")
	require.Empty(t, stdout)
}
