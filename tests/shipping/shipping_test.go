//go:build e2e

// Package shipping holds the e2e tests of `cargueiro ship`, run against the real
// binary, the local registry that asks for no password (see `make registry-up`),
// a local server for the logo and a temporary charts repository. The chart is
// made the way a chart owner makes it: stamped, packaged and pushed.
package shipping

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
)

const (
	chartName    = "rancher-compliance"
	chartVersion = "111.0.0+up1.5.4"
	svg          = `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`
)

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// logoServer serves the logo, counts the requests it gets, and answers status.
type logoServer struct {
	*httptest.Server
	hits atomic.Int32
}

func newLogoServer(t *testing.T, status int) *logoServer {
	t.Helper()

	s := &logoServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.hits.Add(1)
		w.Header().Set("Content-Type", "image/svg+xml")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(svg))
	}))
	t.Cleanup(s.Close)
	return s
}

// registryPath returns a registry path unique to this run, so tests never see
// each other's charts.
func registryPath() string {
	return fmt.Sprintf("e2e/%d", time.Now().UnixNano())
}

// pushCompliance makes the compliance chart with its icon at iconURL, stamps,
// packages and pushes it to a new path of the open registry, and returns the
// registry address, the path and the reference to give to ship.
func pushCompliance(t *testing.T, iconURL string) (address, path, reference string) {
	t.Helper()

	address = harness.OpenRegistry(t)
	session := harness.NewSession(t)
	res := session.Login(t, address, []string{"--plain-http"})
	require.Zero(t, res.ExitCode, res.Stderr)

	src := harness.ChartSource(t, "rancher-compliance-upstream", chartName)
	dir := t.TempDir()
	for _, file := range []string{"Chart.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join(src, file))
		require.NoError(t, err)
		if file == "Chart.yaml" {
			const old = "icon: https://charts.rancher.io/assets/logos/rancher-compliance.svg\n"
			require.Contains(t, string(data), old)
			data = []byte(strings.Replace(string(data), old, "icon: "+iconURL+"\n", 1))
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, file), data, 0o600))
	}

	res = harness.Cargueiro(t, t.TempDir(), "stamp", dir, "--version", chartVersion)
	require.Zero(t, res.ExitCode, res.Stderr)
	res = harness.Cargueiro(t, t.TempDir(), "package", dir, "--output-dir", t.TempDir())
	require.Zero(t, res.ExitCode, res.Stderr)
	archive := strings.TrimSpace(res.Stdout)

	path = registryPath()
	pushed := session.Push(t, address+"/"+path, archive)
	require.Zero(t, pushed.ExitCode, pushed.Stderr)

	return address, path, fmt.Sprintf("%s/%s/%s:%s", address, path, chartName, chartVersion)
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestShipThenIndex(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	address, path, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := t.TempDir()
	entry := filepath.Join(repo, "entries", chartName, chartVersion+".yaml")

	res := harness.Cargueiro(t, t.TempDir(), "ship", reference, "--repo", repo, "--plain-http")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/"+chartName+"/"+chartVersion+".yaml\nassets/logos/"+chartName+".svg\n", res.Stdout)
	require.Equal(t, svg, readFile(t, filepath.Join(repo, "assets", "logos", chartName+".svg")))
	text := readFile(t, entry)
	require.Contains(t, text, "\nicon: file://assets/logos/"+chartName+".svg\n")
	require.Contains(t, text, "\nversion: "+chartVersion+"\n")
	require.Contains(t, text, "\nurls:\n- oci://"+address+"/"+path+"/"+chartName+":"+chartVersion+"@sha256:")
	require.NoDirExists(t, filepath.Join(repo, "assets", chartName), "ship writes no archive")
	require.NoFileExists(t, filepath.Join(repo, "index.yaml"), "and no index")

	// What ship wrote is what index reads.
	index := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo, "--registry", address+"/"+path, "--plain-http")

	require.Zero(t, index.ExitCode, index.Stderr)
	require.Equal(t, "add "+chartName+" "+chartVersion+"\n", index.Stdout)
	require.FileExists(t, filepath.Join(repo, "assets", chartName, chartName+"-"+chartVersion+".tgz"))
	indexText := readFile(t, filepath.Join(repo, "index.yaml"))
	require.Contains(t, indexText, "icon: file://assets/logos/"+chartName+".svg")
	require.Contains(t, indexText, "digest: ")
}

func TestShipNeverReplacesAnEntry(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := t.TempDir()
	first := harness.Cargueiro(t, t.TempDir(), "ship", reference, "--repo", repo, "--plain-http")
	require.Zero(t, first.ExitCode, first.Stderr)
	entry := filepath.Join(repo, "entries", chartName, chartVersion+".yaml")
	before := readFile(t, entry)

	res := harness.Cargueiro(t, t.TempDir(), "ship", reference, "--repo", repo, "--plain-http")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "an entry is never replaced")
	require.Empty(t, res.Stdout)
	require.Equal(t, before, readFile(t, entry))
}

func TestShipDryRunWritesNothing(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := t.TempDir()

	res := harness.Cargueiro(t, t.TempDir(), "ship", reference, "--repo", repo, "--plain-http", "--dry-run")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/"+chartName+"/"+chartVersion+".yaml\nassets/logos/"+chartName+".svg\n", res.Stdout)
	entries, err := os.ReadDir(repo)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestShipUsesTheLogoTheRepositoryHas(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := t.TempDir()
	logos := filepath.Join(repo, "assets", "logos")
	require.NoError(t, os.MkdirAll(logos, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(logos, chartName+".png"), []byte("the logo"), 0o600))

	res := harness.Cargueiro(t, t.TempDir(), "ship", reference, "--repo", repo, "--plain-http")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/"+chartName+"/"+chartVersion+".yaml\n", res.Stdout)
	require.Contains(t, readFile(t, filepath.Join(repo, "entries", chartName, chartVersion+".yaml")), "\nicon: file://assets/logos/"+chartName+".png\n")
	require.Equal(t, "the logo", readFile(t, filepath.Join(logos, chartName+".png")), "the logo is not replaced")
	require.Zero(t, logo.hits.Load(), "nothing is downloaded")
}

func TestShipWritesNothingWhenTheLogoCannotBeDownloaded(t *testing.T) {
	logo := newLogoServer(t, http.StatusNotFound)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := t.TempDir()

	res := harness.Cargueiro(t, t.TempDir(), "ship", reference, "--repo", repo, "--plain-http")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "the server answered 404")
	require.Empty(t, res.Stdout)
	entries, err := os.ReadDir(repo)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestShipRefusesAVersionThatIsNotInTheRegistry(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	address, path, _ := pushCompliance(t, logo.URL+"/compliance")
	repo := t.TempDir()

	res := harness.Cargueiro(t, t.TempDir(), "ship", address+"/"+path+"/"+chartName+":111.0.9+up1.5.9", "--repo", repo, "--plain-http")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "was not found in the registry")
	entries, err := os.ReadDir(repo)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestShipAChartThatIsExempt(t *testing.T) {
	address := harness.OpenRegistry(t)
	session := harness.NewSession(t)
	res := session.Login(t, address, []string{"--plain-http"})
	require.Zero(t, res.ExitCode, res.Stderr)
	path := registryPath()
	pushed := session.Push(t, address+"/"+path, harness.FleetPublished(t, "fleet", "110.0.3+up0.16.3"))
	require.Zero(t, pushed.ExitCode, pushed.Stderr)
	repo := t.TempDir()

	res = harness.Cargueiro(t, t.TempDir(), "ship", address+"/"+path+"/fleet:110.0.3+up0.16.3", "--repo", repo, "--plain-http")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/fleet/110.0.3+up0.16.3.yaml\n", res.Stdout, "no logo")
	text := readFile(t, filepath.Join(repo, "entries", "fleet", "110.0.3+up0.16.3.yaml"))
	require.Contains(t, text, "\nicon: https://charts.rancher.io/assets/logos/fleet.svg\n", "the icon is as it was")
	require.NoDirExists(t, filepath.Join(repo, "assets"))
}
