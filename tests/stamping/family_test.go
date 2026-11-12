//go:build e2e

package stamping

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

// fleetFamily copies the three fleet charts, in the order of the family. The
// fixtures are for Rancher 2.15, which stamp does not support, so the main chart
// is made a chart for Rancher 2.17, whose charts have the major 112.
func fleetFamily(t *testing.T) (fleet, agent, crd string) {
	t.Helper()

	fleet = copyChart(t, "fleet", "fleet")
	edit(t, fleet, "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.17.0-0 < 2.18.0-0")
	return fleet, copyChart(t, "fleet", "fleet-agent"), copyChart(t, "fleet", "fleet-crd")
}

func TestStampFamilyStampsTheMainChart(t *testing.T) {
	fleet, agent, crd := fleetFamily(t)
	agentBefore, crdBefore := readChartYAML(t, agent), readChartYAML(t, crd)

	// Given in the opposite order, to show that the order comes from the rules.
	res := harness.Cargueiro(t, t.TempDir(), "stamp", "--family", "fleet", crd, agent, fleet, "--version", "112.0.0+up0.16.3")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Empty(t, res.Stderr)
	require.Equal(t, "==> fleet\nversion: 110.0.3+up0.16.3 -> 112.0.0+up0.16.3\n", res.Stdout)
	require.Contains(t, readChartYAML(t, fleet), "\nversion: 112.0.0+up0.16.3\n")
	require.Equal(t, agentBefore, readChartYAML(t, agent), "the other charts are not changed")
	require.Equal(t, crdBefore, readChartYAML(t, crd))
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
