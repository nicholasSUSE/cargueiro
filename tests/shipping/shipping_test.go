//go:build e2e

// Package shipping holds the e2e tests of `cargueiro ship`, run against the real
// binary, the local registry that asks for no password (see `make registry-up`),
// a local server for the logo and a temporary charts repository. The chart is
// made the way a chart owner makes it: stamped, packaged and pushed.
package shipping

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
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

	return pushComplianceTo(t, harness.OpenRegistry(t), iconURL)
}

// pushComplianceTo is pushCompliance to the registry at address, which logs in
// as the registry user with plain HTTP.
func pushComplianceTo(t *testing.T, address, iconURL string) (_, path, reference string) {
	t.Helper()

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

// charts returns a checkout like the staging branch of rancher/charts.
func charts(t *testing.T) string {
	t.Helper()

	dir, _ := harness.ChartsCheckout(t)
	return dir
}

// ship runs `cargueiro ship` with the settings of git of whoever runs the tests
// left out, and returns what it printed.
func ship(t *testing.T, args ...string) harness.Result {
	t.Helper()

	return harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: harness.GitEnv()}, append([]string{"ship"}, args...)...)
}

// requireClean fails unless nothing was written to the checkout.
func requireClean(t *testing.T, repo string) {
	t.Helper()

	require.Empty(t, harness.Git(t, repo, "status", "--porcelain"), "the checkout is as it was")
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// createdLine matches the created field of an entry or of a line of the index.
var createdLine = regexp.MustCompile(`(?m)^ *created: "([^"]+)"$`)

// generatedLine matches the generated field of the index.
var generatedLine = regexp.MustCompile(`(?m)^generated: "([^"]+)"$`)

// registryCreated asks the registry itself when the chart at address/repository:tag
// was pushed: the org.opencontainers.image.created annotation of its manifest.
// It is the independent source of the date that ship writes.
func registryCreated(t *testing.T, address, repository, tag string) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "http://"+address+"/v2/"+repository+"/manifests/"+tag, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode, string(body))

	var manifest struct {
		Annotations map[string]string `json:"annotations"`
	}
	require.NoError(t, json.Unmarshal(body, &manifest))
	created := manifest.Annotations["org.opencontainers.image.created"]
	require.NotEmpty(t, created, "the registry gives the date the chart was pushed")
	return created
}

// onlyMatch returns the one group of the only match of re in text.
func onlyMatch(t *testing.T, re *regexp.Regexp, text string) string {
	t.Helper()

	matches := re.FindAllStringSubmatch(text, -1)
	require.Len(t, matches, 1, "%s in %q", re, text)
	return matches[0][1]
}

func TestShipThenIndex(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	address, path, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := charts(t)
	entry := filepath.Join(repo, "entries", chartName, chartVersion+".yaml")

	res := ship(t, reference, "--repo", repo, "--plain-http")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/"+chartName+"/"+chartVersion+".yaml\nassets/logos/"+chartName+".svg\n", res.Stdout)
	require.Equal(t, svg, readFile(t, filepath.Join(repo, "assets", "logos", chartName+".svg")))
	text := readFile(t, entry)
	require.Contains(t, text, "\nicon: file://assets/logos/"+chartName+".svg\n")
	require.Contains(t, text, "\nversion: "+chartVersion+"\n")
	require.Contains(t, text, "\nurls:\n- oci://"+address+"/"+path+"/"+chartName+":"+strings.ReplaceAll(chartVersion, "+", "_")+"@sha256:",
		"the tag of the url is the one the registry stores")
	require.Equal(t, registryCreated(t, address, path+"/"+chartName, strings.ReplaceAll(chartVersion, "+", "_")),
		onlyMatch(t, createdLine, text), "the date is the one the registry gives for the chart")
	require.NoDirExists(t, filepath.Join(repo, "assets", chartName), "ship writes no archive")
	require.Empty(t, readFile(t, filepath.Join(repo, "index.yaml")), "and does not touch the index")

	// What ship wrote is what index reads.
	index := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo, "--registry", address+"/"+path, "--plain-http")

	require.Zero(t, index.ExitCode, index.Stderr)
	require.Equal(t, "add "+chartName+" "+chartVersion+"\n", index.Stdout)
	require.FileExists(t, filepath.Join(repo, "assets", chartName, chartName+"-"+chartVersion+".tgz"))
	indexText := readFile(t, filepath.Join(repo, "index.yaml"))
	require.Contains(t, indexText, "icon: file://assets/logos/"+chartName+".svg")
	require.Contains(t, indexText, "digest: ")
	created := onlyMatch(t, createdLine, text)
	require.Equal(t, created, onlyMatch(t, createdLine, indexText), "the index keeps the date of the entry")
	require.Equal(t, created, onlyMatch(t, generatedLine, indexText), "and says it was generated then, the newest of its versions")
}

func TestShipNeverReplacesAnEntry(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := charts(t)
	first := ship(t, reference, "--repo", repo, "--plain-http")
	require.Zero(t, first.ExitCode, first.Stderr)
	entry := filepath.Join(repo, "entries", chartName, chartVersion+".yaml")
	before := readFile(t, entry)
	// The entry is merged: the checkout is clean and up to date again.
	harness.CommitAndPush(t, repo, "ship "+chartVersion)

	res := ship(t, reference, "--repo", repo, "--plain-http")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "an entry is never replaced")
	require.Empty(t, res.Stdout)
	require.Equal(t, before, readFile(t, entry))
}

func TestShipDryRunWritesNothing(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := charts(t)

	res := ship(t, reference, "--repo", repo, "--plain-http", "--dry-run")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/"+chartName+"/"+chartVersion+".yaml\nassets/logos/"+chartName+".svg\n", res.Stdout)
	requireClean(t, repo)
}

func TestShipUsesTheLogoTheRepositoryHas(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := charts(t)
	logos := filepath.Join(repo, "assets", "logos")
	require.NoError(t, os.MkdirAll(logos, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(logos, chartName+".png"), []byte("the logo"), 0o600))
	harness.CommitAndPush(t, repo, "the logo")

	res := ship(t, reference, "--repo", repo, "--plain-http")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/"+chartName+"/"+chartVersion+".yaml\n", res.Stdout)
	require.Contains(t, readFile(t, filepath.Join(repo, "entries", chartName, chartVersion+".yaml")), "\nicon: file://assets/logos/"+chartName+".png\n")
	require.Equal(t, "the logo", readFile(t, filepath.Join(logos, chartName+".png")), "the logo is not replaced")
	require.Zero(t, logo.hits.Load(), "nothing is downloaded")
}

func TestShipWritesNothingWhenTheLogoCannotBeDownloaded(t *testing.T) {
	logo := newLogoServer(t, http.StatusNotFound)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := charts(t)

	res := ship(t, reference, "--repo", repo, "--plain-http")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "the server answered 404")
	require.Empty(t, res.Stdout)
	requireClean(t, repo)
}

func TestShipRefusesAVersionThatIsNotInTheRegistry(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	address, path, _ := pushCompliance(t, logo.URL+"/compliance")
	repo := charts(t)

	res := ship(t, address+"/"+path+"/"+chartName+":111.0.9+up1.5.9", "--repo", repo, "--plain-http")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "was not found in the registry")
	requireClean(t, repo)
}

func TestShipAChartThatIsExempt(t *testing.T) {
	address := harness.OpenRegistry(t)
	session := harness.NewSession(t)
	res := session.Login(t, address, []string{"--plain-http"})
	require.Zero(t, res.ExitCode, res.Stderr)
	path := registryPath()
	pushed := session.Push(t, address+"/"+path, harness.FleetPublished(t, "fleet", "110.0.3+up0.16.3"))
	require.Zero(t, pushed.ExitCode, pushed.Stderr)
	repo := charts(t)

	res = harness.Cargueiro(t, t.TempDir(), "ship", address+"/"+path+"/fleet:110.0.3+up0.16.3", "--repo", repo, "--plain-http")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "entries/fleet/110.0.3+up0.16.3.yaml\n", res.Stdout, "no logo")
	text := readFile(t, filepath.Join(repo, "entries", "fleet", "110.0.3+up0.16.3.yaml"))
	require.Contains(t, text, "\nicon: https://charts.rancher.io/assets/logos/fleet.svg\n", "the icon is as it was")
	require.NoDirExists(t, filepath.Join(repo, "assets"))
}

func TestShipRefusesACheckoutThatIsNotRight(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		args    []string
		wantErr string
	}{
		{
			name: "something that is not committed",
			setup: func(t *testing.T) string {
				repo := charts(t)
				require.NoError(t, os.WriteFile(filepath.Join(repo, "notes.txt"), []byte("x"), 0o600))
				return repo
			},
			wantErr: "has changes that are not committed",
		},
		{
			name: "a commit that is not pushed",
			setup: func(t *testing.T) string {
				repo := charts(t)
				require.NoError(t, os.WriteFile(filepath.Join(repo, "notes.txt"), []byte("x"), 0o600))
				harness.Git(t, repo, "add", "-A")
				harness.Git(t, repo, "commit", "-m", "local")
				return repo
			},
			wantErr: "upstream: update it",
		},
		{
			name: "behind the remote",
			setup: func(t *testing.T) string {
				repo, remote := harness.ChartsCheckout(t)
				other := filepath.Join(t.TempDir(), "other")
				harness.Git(t, t.TempDir(), "clone", "--branch", "staging", remote, other)
				require.NoError(t, os.WriteFile(filepath.Join(other, "merged.txt"), []byte("x"), 0o600))
				harness.CommitAndPush(t, other, "merged by someone else")
				return repo
			},
			wantErr: "upstream: update it",
		},
		{
			name: "another branch",
			setup: func(t *testing.T) string {
				repo := charts(t)
				harness.Git(t, repo, "checkout", "-b", "feature")
				return repo
			},
			wantErr: "is on the branch feature, and it must be on staging: check it out with git checkout staging",
		},
		{
			name: "no upstream",
			setup: func(t *testing.T) string {
				repo := charts(t)
				harness.Git(t, repo, "branch", "--unset-upstream")
				return repo
			},
			wantErr: "staging has no upstream. The remotes are: origin (",
		},
		{
			name: "no remote",
			setup: func(t *testing.T) string {
				repo := charts(t)
				harness.Git(t, repo, "remote", "remove", "origin")
				return repo
			},
			wantErr: "has no git remote",
		},
		{
			name: "not a checkout",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: "is not a git checkout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setup(t)

			res := ship(t, reference, "--repo", repo, "--plain-http")

			require.NotZero(t, res.ExitCode)
			require.Contains(t, res.Stderr, tt.wantErr)
			require.Empty(t, res.Stdout)
			require.NoDirExists(t, filepath.Join(repo, "entries"))
			require.NoDirExists(t, filepath.Join(repo, "assets"))
		})
	}
}

func TestShipChecksTheCheckoutOnADryRunToo(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := charts(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "notes.txt"), []byte("x"), 0o600))

	res := ship(t, reference, "--repo", repo, "--plain-http", "--dry-run")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "has changes that are not committed")
	require.Zero(t, logo.hits.Load(), "and the logo is not downloaded")
}

func TestShipTakesAnotherBranch(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	_, _, reference := pushCompliance(t, logo.URL+"/compliance")
	repo := charts(t)
	harness.Git(t, repo, "checkout", "-b", "release")
	harness.Git(t, repo, "push", "--set-upstream", "origin", "release")

	res := ship(t, reference, "--repo", repo, "--plain-http", "--branch", "release")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "entries/"+chartName+"/"+chartVersion+".yaml")
}
