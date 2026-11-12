//go:build e2e

package versioning

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

// fleetFamily copies the three fleet charts, in the order of the family. The
// fixtures are for Rancher 2.15, which is not supported, so the main chart and
// the extra one are made charts for Rancher 2.17, whose charts have the major
// 112. The CRD chart has no rancher-version.
func fleetFamily(t *testing.T) (fleet, agent, crd string) {
	t.Helper()

	fleet = copyChart(t, "fleet", "fleet")
	edit(t, fleet, "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.17.0-0 < 2.18.0-0")
	agent = copyChart(t, "fleet", "fleet-agent")
	edit(t, agent, "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.17.0-0 < 2.18.0-0")
	return fleet, agent, copyChart(t, "fleet", "fleet-crd")
}

// TestChartVersionOfAFamilyFollowsItsReleases releases the fleet family to the
// local registry: the version of the family is worked out from its main chart,
// the family is stamped, packaged and pushed with it, and the next version must
// follow. When only the main chart is released again, the family has drifted and
// the version is refused.
func TestChartVersionOfAFamilyFollowsItsReleases(t *testing.T) {
	address := harness.OpenRegistry(t)
	session := harness.NewSession(t)
	login := session.Login(t, address, []string{"--plain-http"})
	require.Zero(t, login.ExitCode, login.Stderr)
	path := fmt.Sprintf("e2e/%d", time.Now().UnixNano())
	registry := []string{"--registry", address + "/" + path, "--plain-http"}
	fleet, agent, crd := fleetFamily(t)

	// Given in the opposite order of the family, to show that it does not matter.
	familyVersion := func(t *testing.T) harness.Result {
		t.Helper()

		args := append([]string{"chart-version", "--family", "fleet", crd, agent, fleet}, registry...)
		return harness.Cargueiro(t, t.TempDir(), args...)
	}

	t.Run("a family that was never pushed starts at the first version", func(t *testing.T) {
		res := familyVersion(t)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "112.0.0+up0.16.3\n", res.Stdout)
	})

	t.Run("the family is released with that version, and the next one follows", func(t *testing.T) {
		want := strings.TrimSpace(familyVersion(t).Stdout)
		stamp := harness.Cargueiro(t, t.TempDir(), "stamp", "--family", "fleet", crd, agent, fleet, "--version", want)
		require.Zero(t, stamp.ExitCode, stamp.Stderr)
		pkg := harness.Cargueiro(t, t.TempDir(), "package", "--family", "fleet", crd, agent, fleet, "--output-dir", t.TempDir())
		require.Zero(t, pkg.ExitCode, pkg.Stderr)
		archives := strings.Fields(pkg.Stdout)
		require.Len(t, archives, 3)
		push := session.Run(t, "", nil, append(append([]string{"push", "--family", "fleet"}, archives...), "--registry", address+"/"+path)...)
		require.Zero(t, push.ExitCode, push.Stderr)

		res := familyVersion(t)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "112.0.1+up0.16.3\n", res.Stdout)
	})

	t.Run("a main chart released alone makes the family drift", func(t *testing.T) {
		alone, _, _ := fleetFamily(t)
		stamp := harness.Cargueiro(t, t.TempDir(), "stamp", alone, "--version", "112.0.1+up0.16.3")
		require.Zero(t, stamp.ExitCode, stamp.Stderr)
		pkg := harness.Cargueiro(t, t.TempDir(), "package", alone, "--output-dir", t.TempDir())
		require.Zero(t, pkg.ExitCode, pkg.Stderr)
		push := session.Push(t, address+"/"+path, strings.TrimSpace(pkg.Stdout))
		require.Zero(t, push.ExitCode, push.Stderr)

		res := familyVersion(t)

		require.Equal(t, 1, res.ExitCode)
		require.Empty(t, res.Stdout)
		require.Contains(t, res.Stderr, `the charts of the family "fleet" are not at the same released version: `+
			"fleet 112.0.1+up0.16.3, fleet-agent 112.0.0+up0.16.3, fleet-crd 112.0.0+up0.16.3")
	})
}

func TestChartVersionOfAFamilyIsRefusedBeforeAnyConnection(t *testing.T) {
	fleet, agent, crd := fleetFamily(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "a chart is missing",
			args:    []string{"--family", "fleet", fleet, crd},
			wantErr: `the family "fleet" is missing fleet-agent`,
		},
		{
			name:    "a chart is given twice",
			args:    []string{"--family", "fleet", fleet, fleet, agent, crd},
			wantErr: `chart "fleet" is given twice`,
		},
		{
			name:    "a family that is not listed",
			args:    []string{"--family", "nope", fleet},
			wantErr: `family "nope" is not listed in the rules file`,
		},
		{
			name:    "a folder with no family",
			args:    []string{fleet, agent},
			wantErr: "accepts 1 arg(s), received 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: deadProxy()},
				append([]string{"chart-version"}, tt.args...)...)

			require.Equal(t, 1, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
		})
	}
}
