//go:build e2e

// Package listing holds the e2e tests of `cargueiro list`, run against the real
// binary and the local registry that asks for no password (see
// `make registry-up`). Charts are pushed to it in a scrambled order, and the
// listing must put them in the right one.
package listing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
)

const fleetVersion = "110.0.3+up0.16.3"

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// repoPath returns a registry path unique to this run, so tests never see each
// other's charts.
func repoPath() string {
	return fmt.Sprintf("e2e/%d", time.Now().UnixNano())
}

// openRegistry logs a session in to the registry that asks for no password and
// returns its address and the session, which pushes there.
func openRegistry(t *testing.T) (string, harness.Session) {
	t.Helper()

	address := harness.OpenRegistry(t)
	session := harness.NewSession(t)
	res := session.Login(t, address, []string{"--plain-http"})
	require.Zero(t, res.ExitCode, res.Stderr)
	return address, session
}

// withVersion copies Chart.yaml and values.yaml of a chart directory and
// changes the version of the copy. The chart is the real one; the version is
// the only difference.
func withVersion(t *testing.T, src, from, to string) string {
	t.Helper()

	dst := t.TempDir()
	for _, file := range []string{"Chart.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join(src, file))
		require.NoError(t, err)
		if file == "Chart.yaml" {
			changed := strings.Replace(string(data), "\nversion: "+from+"\n", "\nversion: "+to+"\n", 1)
			require.Contains(t, changed, "\nversion: "+to+"\n", "Chart.yaml has no version %s", from)
			data = []byte(changed)
		}
		require.NoError(t, os.WriteFile(filepath.Join(dst, file), data, 0o600))
	}
	return dst
}

// pushVersions packages the chart with each of the versions and pushes them,
// in the order given, to <address>/<path>. The Helm CLI packages them, because
// the versions are chosen to test the order, and cargueiro does not package a
// chart whose version does not match its rancher-version.
func pushVersions(t *testing.T, session harness.Session, address, path, src, from string, versions []string) {
	t.Helper()

	for _, v := range versions {
		out := t.TempDir()
		pkg := harness.Helm(t, t.TempDir(), "package", withVersion(t, src, from, v), "--destination", out)
		require.Zero(t, pkg.ExitCode, pkg.Stderr)
		archives, err := filepath.Glob(filepath.Join(out, "*.tgz"))
		require.NoError(t, err)
		require.Len(t, archives, 1)

		res := session.Push(t, address+"/"+path, archives[0])
		require.Zero(t, res.ExitCode, res.Stderr)
	}
}

// list runs list on a chart in the open registry.
func list(t *testing.T, repository string, args ...string) harness.Result {
	t.Helper()

	return harness.Cargueiro(t, t.TempDir(), append([]string{"list", repository, "--plain-http"}, args...)...)
}

// registryTags reads the tags of a repository straight from the registry,
// without cargueiro, to check the listing against.
func registryTags(t *testing.T, address, repository string) []string {
	t.Helper()

	resp, err := http.Get("http://" + address + "/v2/" + repository + "/tags/list")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Tags []string `json:"tags"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	sort.Strings(body.Tags)
	return body.Tags
}

func TestListFleetNewestFirst(t *testing.T) {
	address, session := openRegistry(t)
	path := repoPath()
	// Pushed in an order that is not the right one.
	pushVersions(t, session, address, path, harness.FleetSource(t, "fleet"), fleetVersion, []string{
		"110.0.3+up0.16.3",
		"110.0.10+up0.16.3",
		"109.0.9+up1.0.0",
		"110.0.3+up0.16.3-rc.1",
		"110.0.9+up0.16.3",
		"110.0.3+up0.16.10",
		"110.1.0+up0.1.0",
		"110.0.3+up0.16.3-rancher.2",
	})
	repository := address + "/" + path + "/fleet"
	want := []string{
		"110.1.0+up0.1.0",
		"110.0.10+up0.16.3",
		"110.0.9+up0.16.3",
		"110.0.3+up0.16.10",
		"110.0.3+up0.16.3",
		"110.0.3+up0.16.3-rancher.2",
		"110.0.3+up0.16.3-rc.1",
		"109.0.9+up1.0.0",
	}

	t.Run("text", func(t *testing.T) {
		res := list(t, repository)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Empty(t, res.Stderr)
		require.Equal(t, strings.Join(want, "\n")+"\n", res.Stdout)
	})

	t.Run("json", func(t *testing.T) {
		res := list(t, repository, "--output", "json")

		require.Zero(t, res.ExitCode, res.Stderr)
		var got []struct {
			Version string `json:"version"`
			Tag     string `json:"tag"`
		}
		require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got), res.Stdout)
		require.Len(t, got, len(want))
		for i, v := range want {
			require.Equal(t, v, got[i].Version)
			require.Equal(t, strings.ReplaceAll(v, "+", "_"), got[i].Tag)
		}
	})

	t.Run("it lists the tags the registry has", func(t *testing.T) {
		res := list(t, repository, "--output", "json")
		require.Zero(t, res.ExitCode, res.Stderr)

		var got []struct {
			Tag string `json:"tag"`
		}
		require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got))
		tags := make([]string, len(got))
		for i, v := range got {
			tags[i] = v.Tag
		}
		sort.Strings(tags)
		require.Equal(t, registryTags(t, address, path+"/fleet"), tags)
	})

	t.Run("the oci scheme", func(t *testing.T) {
		res := list(t, "oci://"+repository)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, strings.Join(want, "\n")+"\n", res.Stdout)
	})
}

func TestListPlainChart(t *testing.T) {
	address, session := openRegistry(t)
	path := repoPath()
	src := harness.ChartSource(t, "system-upgrade-controller", "system-upgrade-controller")
	pushVersions(t, session, address, path, src, "110.0.0", []string{
		"110.0.9",
		"110.0.0",
		"111.0.0",
		"110.0.10",
	})

	res := list(t, address+"/"+path+"/system-upgrade-controller")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "111.0.0\n110.0.10\n110.0.9\n110.0.0\n", res.Stdout)
}

func TestListFailsWithoutPrinting(t *testing.T) {
	address, session := openRegistry(t)
	path := repoPath()
	// A fleet version without a +up part is valid for Helm, but not a Rancher one.
	pushVersions(t, session, address, path, harness.FleetSource(t, "fleet"), fleetVersion, []string{
		"110.0.3+up0.16.3",
		"110.0.3",
	})

	tests := []struct {
		name       string
		repository string
		args       []string
		wantErr    string
	}{
		{
			name:       "a repository that does not exist",
			repository: address + "/" + repoPath() + "/fleet",
			wantErr:    "was not found in the registry",
		},
		{
			name:       "a version that does not follow the rules",
			repository: address + "/" + path + "/fleet",
			wantErr:    `version "110.0.3" must look like`,
		},
		{
			name:       "a chart that is not in the rules file",
			repository: address + "/" + path + "/epinio",
			wantErr:    `chart "epinio" is not listed in the rules file`,
		},
		{
			name:       "a reference with a tag",
			repository: address + "/" + path + "/fleet:110.0.3_up0.16.3",
			wantErr:    "a tag is not accepted",
		},
		{
			name:       "an unknown output format",
			repository: address + "/" + path + "/fleet",
			args:       []string{"--output", "yaml"},
			wantErr:    `unknown output format "yaml"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := list(t, tt.repository, tt.args...)

			require.Equal(t, 1, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
		})
	}
}

func TestListNeedsPlainHTTPForALocalRegistry(t *testing.T) {
	address, session := openRegistry(t)
	path := repoPath()
	pushVersions(t, session, address, path, harness.FleetSource(t, "fleet"), fleetVersion, []string{fleetVersion})

	res := harness.Cargueiro(t, t.TempDir(), "list", address+"/"+path+"/fleet")

	require.Equal(t, 1, res.ExitCode)
	require.Empty(t, res.Stdout)
}
