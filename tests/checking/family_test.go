//go:build e2e

package checking

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/check"
	"github.com/rancher/cargueiro/tests/internal/harness"
)

// fleetFamily is the fleet family, the main chart first, then the order of rules.yaml.
func fleetFamily() []string {
	return []string{"fleet", "fleet-agent", "fleet-crd"}
}

func TestCheckChartFamilyOfFleet(t *testing.T) {
	// Given in the opposite order, to show that the order comes from the rules.
	args := []string{"check", "chart", "--family", "fleet", "--output", "json"}
	family := fleetFamily()
	for i := len(family) - 1; i >= 0; i-- {
		args = append(args, harness.FleetSource(t, family[i]))
	}

	res := harness.Cargueiro(t, t.TempDir(), args...)

	require.Zero(t, res.ExitCode, res.Stderr)
	var got []check.Result
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got), res.Stdout)
	require.Len(t, got, 3)
	for i, want := range fleetFamily() {
		require.True(t, got[i].Valid, got[i].Chart)
		require.Equal(t, want, got[i].Chart, "the main chart first, then the order of rules.yaml")
	}
}

func TestCheckArchiveFamilyOfFleet(t *testing.T) {
	args := []string{"check", "archive", "--family", "fleet"}
	for _, name := range fleetFamily() {
		args = append(args, harness.FleetPublished(t, name, fleetVersion))
	}

	res := harness.Cargueiro(t, t.TempDir(), args...)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, 3, strings.Count(res.Stdout, "valid: true\n"))
	require.Empty(t, res.Stderr)
}

func TestCheckChartFamilyRefusesAnIncompleteFamily(t *testing.T) {
	res := harness.Cargueiro(t, t.TempDir(), "check", "chart", "--family", "fleet",
		harness.FleetSource(t, "fleet"), harness.FleetSource(t, "fleet-crd"))

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, `the family "fleet" is missing fleet-agent`)
	require.Empty(t, res.Stdout)
}
