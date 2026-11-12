package stamp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/oci"
)

// fleetFamily copies the three fleet charts, in the order of the family. The
// fixtures are for Rancher 2.15, which stamp does not support, so the main chart
// is made a chart for Rancher 2.17, whose charts have the major 112.
func fleetFamily(t *testing.T) []string {
	t.Helper()

	dirs := []string{
		copyChart(t, "fleet", "fleet"),
		copyChart(t, "fleet", "fleet-agent"),
		copyChart(t, "fleet", "fleet-crd"),
	}
	edit(t, dirs[0], "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.17.0-0 < 2.18.0-0")
	return dirs
}

func readChart(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	return string(data)
}

func TestBuildFamilyStampsTheMainChart(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)
	agentBefore, crdBefore := readChart(t, dirs[1]), readChart(t, dirs[2])

	plan, err := BuildFamily(t.Context(), cfg, members, dirs, Options{Version: "112.0.0+up0.16.3"})

	require.NoError(t, err)
	require.Equal(t, "112.0.0+up0.16.3", plan.Version, "the version of the main chart is the version of the family")
	require.Len(t, plan.Charts, 1, "only the main chart is stamped")
	require.Equal(t, "fleet", plan.Charts[0].Chart)
	require.Equal(t, "110.0.3+up0.16.3", plan.Charts[0].Plan.OldVersion)
	require.Equal(t, "112.0.0+up0.16.3", plan.Charts[0].Plan.NewVersion)

	require.NoError(t, plan.Write())
	require.Contains(t, readChart(t, dirs[0]), "version: 112.0.0+up0.16.3")
	require.Equal(t, agentBefore, readChart(t, dirs[1]), "the other charts are not changed")
	require.Equal(t, crdBefore, readChart(t, dirs[2]))
}

func TestBuildFamilyWorksOutTheVersionFromTheMainChart(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	registry := &fakeRegistry{tags: []string{"112.0.0_up0.16.1"}}

	plan, err := BuildFamily(t.Context(), cfg, members, fleetFamily(t), Options{List: registry.list})

	require.NoError(t, err)
	require.Equal(t, "112.0.1+up0.16.3", plan.Version)
	require.Equal(t, []oci.ChartRepo{{Host: "stgregistry.suse.com", Repository: "rancher/catalog/fleet"}}, registry.asked,
		"only the registry of the main chart is read")
}

func TestBuildFamilyWritesNothingBeforeWrite(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)
	before := readChart(t, dirs[0])

	_, err = BuildFamily(t.Context(), cfg, members, dirs, Options{Version: "112.0.0+up0.16.3"})

	require.NoError(t, err)
	require.Equal(t, before, readChart(t, dirs[0]))
}

func TestBuildFamilyRefusesTheMainChart(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)
	before := readChart(t, dirs[0])

	// The main chart is for Rancher 2.17, which ties to the major 112.
	_, err = BuildFamily(t.Context(), cfg, members, dirs, Options{Version: "111.0.0+up0.16.3"})

	require.ErrorContains(t, err, "fleet: ")
	require.Equal(t, before, readChart(t, dirs[0]))
}

func TestBuildFamilyNeedsAFolderForEveryChart(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)

	_, err = BuildFamily(t.Context(), cfg, members, fleetFamily(t)[:2], Options{Version: "112.0.0+up0.16.3"})

	require.EqualError(t, err, `the family "fleet" has 3 charts and 2 folders were given`)
}
