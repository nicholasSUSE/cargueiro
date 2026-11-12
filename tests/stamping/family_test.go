//go:build e2e

package stamping

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

// fleetFamily copies the three fleet charts, in the order of the family. The
// fixtures are for Rancher 2.15, which stamp does not support, so the main chart
// and the extra one are made charts for Rancher 2.17, whose charts have the
// major 112. The CRD chart has no rancher-version.
func fleetFamily(t *testing.T) (fleet, agent, crd string) {
	t.Helper()

	fleet = copyChart(t, "fleet", "fleet")
	edit(t, fleet, "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.17.0-0 < 2.18.0-0")
	agent = copyChart(t, "fleet", "fleet-agent")
	edit(t, agent, "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.17.0-0 < 2.18.0-0")
	return fleet, agent, copyChart(t, "fleet", "fleet-crd")
}

func TestStampFamilyStampsEveryChart(t *testing.T) {
	fleet, agent, crd := fleetFamily(t)

	// Given in the opposite order, to show that the order comes from the rules,
	// and with no network, because the version is given.
	res := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: deadProxy()},
		"stamp", "--family", "fleet", crd, agent, fleet, "--version", "112.0.0+up0.16.3")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Empty(t, res.Stderr)
	require.Equal(t, "==> fleet\nversion: 110.0.3+up0.16.3 -> 112.0.0+up0.16.3\n"+
		"==> fleet-agent\nversion: 110.0.3+up0.16.3 -> 112.0.0+up0.16.3\n"+
		"==> fleet-crd\nversion: 110.0.3+up0.16.3 -> 112.0.0+up0.16.3\n", res.Stdout)

	// The family that was stamped is a family that check accepts: one version.
	checked := harness.Cargueiro(t, t.TempDir(), "check", "chart", "--family", "fleet", fleet, agent, crd)
	require.Zero(t, checked.ExitCode, checked.Stdout+checked.Stderr)
}

func TestStampFamilyRefusesAnIncompleteFamily(t *testing.T) {
	fleet, _, crd := fleetFamily(t)
	before := readChartYAML(t, fleet)

	res := harness.Cargueiro(t, t.TempDir(), "stamp", "--family", "fleet", fleet, crd, "--version", "112.0.0+up0.16.3")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, `the family "fleet" is missing fleet-agent`)
	require.Empty(t, res.Stdout)
	require.Equal(t, before, readChartYAML(t, fleet), "nothing is changed")
}

func TestStampFamilyWithoutAVersionNeedsTheRegistries(t *testing.T) {
	fleet, agent, crd := fleetFamily(t)
	before := readChartYAML(t, fleet)

	res := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: deadProxy()},
		"stamp", "--family", "fleet", fleet, agent, crd)

	require.Equal(t, 1, res.ExitCode)
	require.Empty(t, res.Stdout)
	require.Contains(t, res.Stderr, "stgregistry.suse.com/rancher/catalog/fleet")
	require.Equal(t, before, readChartYAML(t, fleet))
}
