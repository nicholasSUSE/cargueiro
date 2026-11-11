//go:build e2e

package shipping

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

const fleetVersion = "110.0.3+up0.16.3"

// pushFleet pushes the published archives of the given charts of the fleet
// family to a new path of the open registry, and returns its address and path.
func pushFleet(t *testing.T, charts ...string) (address, path string) {
	t.Helper()

	address = harness.OpenRegistry(t)
	session := harness.NewSession(t)
	res := session.Login(t, address, []string{"--plain-http"})
	require.Zero(t, res.ExitCode, res.Stderr)

	path = registryPath()
	for _, name := range charts {
		pushed := session.Push(t, address+"/"+path, harness.FleetPublished(t, name, fleetVersion))
		require.Zero(t, pushed.ExitCode, pushed.Stderr)
	}
	return address, path
}

func TestShipFamilyThenIndex(t *testing.T) {
	address, path := pushFleet(t, "fleet", "fleet-agent", "fleet-crd")
	repo := charts(t)

	res := ship(t, "--family", "fleet", "--version", fleetVersion, "--registry", address+"/"+path, "--repo", repo, "--plain-http")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/fleet/"+fleetVersion+".yaml\nentries/fleet-agent/"+fleetVersion+".yaml\nentries/fleet-crd/"+fleetVersion+".yaml\n", res.Stdout,
		"the main chart first, then the order of rules.yaml")
	for _, name := range []string{"fleet", "fleet-agent", "fleet-crd"} {
		text := readFile(t, repo+"/entries/"+name+"/"+fleetVersion+".yaml")
		require.Contains(t, text, "\nversion: "+fleetVersion+"\n")
		require.Contains(t, text, "\nurls:\n- oci://"+address+"/"+path+"/"+name+":"+fleetVersion+"@sha256:")
	}

	// What ship wrote is what index reads, and one run indexes the whole family.
	index := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo, "--registry", address+"/"+path, "--plain-http")

	require.Zero(t, index.ExitCode, index.Stderr)
	for _, name := range []string{"fleet", "fleet-agent", "fleet-crd"} {
		require.Contains(t, index.Stdout, "add "+name+" "+fleetVersion+"\n")
		require.FileExists(t, repo+"/assets/"+name+"/"+name+"-"+fleetVersion+".tgz")
	}
}

func TestShipFamilyWritesNothingWhenAChartIsNotInTheRegistry(t *testing.T) {
	address, path := pushFleet(t, "fleet", "fleet-crd")
	repo := charts(t)

	res := ship(t, "--family", "fleet", "--version", fleetVersion, "--registry", address+"/"+path, "--repo", repo, "--plain-http")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "fleet-agent")
	require.Empty(t, res.Stdout)
	requireClean(t, repo)
}

func TestShipFamilyNeverReplacesAnEntry(t *testing.T) {
	address, path := pushFleet(t, "fleet", "fleet-agent", "fleet-crd")
	repo := charts(t)
	args := []string{"--family", "fleet", "--version", fleetVersion, "--registry", address + "/" + path, "--repo", repo, "--plain-http"}
	first := ship(t, args...)
	require.Zero(t, first.ExitCode, first.Stderr)
	// The entries are merged: the checkout is clean and up to date again.
	harness.CommitAndPush(t, repo, "ship fleet")

	again := ship(t, args...)

	require.NotZero(t, again.ExitCode)
	require.Contains(t, again.Stderr, "an entry is never replaced")
	require.Empty(t, again.Stdout)
}
