//go:build e2e

package packaging

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

func TestPackageFamilyOfFleet(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	// Given in the opposite order of the family, to show that the order comes
	// from the rules.
	args := []string{"package", "--family", "fleet", "--output-dir", out}
	for _, name := range fleetCharts() {
		args = append(args, harness.FleetSource(t, name))
	}

	res := harness.Cargueiro(t, t.TempDir(), args...)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Empty(t, res.Stderr)
	var want []string
	for _, name := range []string{"fleet", "fleet-agent", "fleet-crd"} {
		want = append(want, filepath.Join(out, name+"-"+fleetVersion+".tgz"))
	}
	require.Equal(t, strings.Join(want, "\n")+"\n", res.Stdout, "the main chart first, then the order of rules.yaml")

	// The archives are the ones check accepts as a family.
	checked := harness.Cargueiro(t, t.TempDir(), append([]string{"check", "archive", "--family", "fleet"}, want...)...)
	require.Zero(t, checked.ExitCode, checked.Stdout+checked.Stderr)
}

func TestPackageFamilyBuildsNothingForAnIncompleteFamily(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")

	res := harness.Cargueiro(t, t.TempDir(), "package", "--family", "fleet", "--output-dir", out,
		harness.FleetSource(t, "fleet"), harness.FleetSource(t, "fleet-crd"))

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, `the family "fleet" is missing fleet-agent`)
	require.Empty(t, res.Stdout)
	require.NoDirExists(t, out)
}
