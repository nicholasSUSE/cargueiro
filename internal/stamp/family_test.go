package stamp

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// fleetFamily copies the three fleet charts, in the order of the family. The
// fixtures are for Rancher 2.15, which stamp does not support, so the main chart
// and the extra one are made charts for Rancher 2.17, whose charts have the
// major 112. The CRD chart has no rancher-version.
func fleetFamily(t *testing.T) []string {
	t.Helper()

	dirs := []string{
		copyChart(t, "fleet", "fleet"),
		copyChart(t, "fleet", "fleet-agent"),
		copyChart(t, "fleet", "fleet-crd"),
	}
	edit(t, dirs[0], "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.17.0-0 < 2.18.0-0")
	edit(t, dirs[1], "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.17.0-0 < 2.18.0-0")
	return dirs
}

func readChart(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	return string(data)
}

func TestBuildFamilyStampsEveryChart(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)

	plan, err := BuildFamily(cfg, members, dirs, Options{Version: "112.0.0+up0.16.3"})

	require.NoError(t, err)
	require.Equal(t, "112.0.0+up0.16.3", plan.Version, "the version of the main chart is the version of the family")
	var names []string
	for _, chart := range plan.Charts {
		names = append(names, chart.Chart)
		require.Equal(t, "112.0.0+up0.16.3", chart.Plan.NewVersion, chart.Chart)
	}
	require.Equal(t, []string{"fleet", "fleet-agent", "fleet-crd"}, names, "the main chart first, then the order of the rules")

	require.NoError(t, plan.Write())
	for _, dir := range dirs {
		require.Contains(t, readChart(t, dir), "\nversion: 112.0.0+up0.16.3\n")
	}
}

func TestBuildFamilyReportsEveryProblem(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)
	// The extra chart is for Rancher 2.16, whose charts have the major 111, and
	// the CRD chart has no version at all.
	edit(t, dirs[1], "Chart.yaml", ">= 2.17.0-0 < 2.18.0-0", ">= 2.16.0-0 < 2.17.0-0")
	edit(t, dirs[2], "Chart.yaml", "version: 110.0.3+up0.16.3\n", "")

	plan, err := BuildFamily(cfg, members, dirs, Options{Version: "112.0.0+up0.16.3"})

	require.Nil(t, plan)
	require.ErrorContains(t, err, "fleet-agent: ", "the chart that cannot take the version is named")
	require.ErrorContains(t, err, "fleet-crd: ", "and so is the next one")
}

func TestBuildFamilyWritesNothingBeforeWrite(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)
	var before []string
	for _, dir := range dirs {
		before = append(before, readChart(t, dir))
	}

	_, err = BuildFamily(cfg, members, dirs, Options{Version: "112.0.0+up0.16.3"})

	require.NoError(t, err)
	for i, dir := range dirs {
		require.Equal(t, before[i], readChart(t, dir))
	}
}

func TestBuildFamilyRefusesTheMainChart(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)
	before := readChart(t, dirs[0])

	// The main chart is for Rancher 2.17, which ties to the major 112.
	_, err = BuildFamily(cfg, members, dirs, Options{Version: "111.0.0+up0.16.3"})

	require.ErrorContains(t, err, "fleet: ")
	require.Equal(t, before, readChart(t, dirs[0]))
}

func TestBuildFamilyNeedsAFolderForEveryChart(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)

	_, err = BuildFamily(cfg, members, fleetFamily(t)[:2], Options{Version: "112.0.0+up0.16.3"})

	require.EqualError(t, err, `the family "fleet" has 3 charts and 2 folders were given`)
}

func TestFamilyWritePutsBackWhatItWroteWhenALaterWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a folder cannot be made read-only for root")
	}
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)
	var before []string
	for _, dir := range dirs {
		before = append(before, readChart(t, dir))
	}
	plan, err := BuildFamily(cfg, members, dirs, Options{Version: "112.0.0+up0.16.3"})
	require.NoError(t, err)

	// The last chart cannot be written, so the two before it have to be put back.
	require.NoError(t, os.Chmod(dirs[2], 0o500))
	t.Cleanup(func() { _ = os.Chmod(dirs[2], 0o700) })
	err = plan.Write()

	require.ErrorContains(t, err, "fleet-crd: ")
	require.ErrorContains(t, err, "were put back as they were")
	for i, dir := range dirs {
		require.Equal(t, before[i], readChart(t, dir), "chart %d is as it was", i)
	}
}

func TestFamilyWriteSaysSoWhenItCannotPutBackWhatItWrote(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a folder cannot be made read-only for root")
	}
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	dirs := fleetFamily(t)
	plan, err := BuildFamily(cfg, members, dirs, Options{Version: "112.0.0+up0.16.3"})
	require.NoError(t, err)
	plan.undo = func([]*Plan) error { return errors.New("the disk is gone") }

	require.NoError(t, os.Chmod(dirs[2], 0o500))
	t.Cleanup(func() { _ = os.Chmod(dirs[2], 0o700) })
	err = plan.Write()

	require.ErrorContains(t, err, "fleet-crd: ")
	require.ErrorContains(t, err, "the disk is gone")
	require.ErrorContains(t, err, "something catastrophic happened")
	require.ErrorContains(t, err, "git reset --hard")
}
