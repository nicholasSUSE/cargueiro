//go:build e2e

package indexing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
)

// chartRef names one version of one chart of the fixture.
type chartRef struct {
	chart   string
	version string
}

// fixedTime is the time of the files of every chart, so that Helm makes the
// same archive on every run.
func fixedTime() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

// pushedChart is a chart of the fixture, once it is in the registry.
type pushedChart struct {
	// archiveDigest is the sha256 of the archive Helm made, in hex.
	archiveDigest string
	// url is where the chart is pulled from, pinned to its manifest.
	url string
}

var (
	urlLine        = regexp.MustCompile(`(?m)^urls:\n- .*\n`)
	manifestDigest = regexp.MustCompile(`Digest: (sha256:[0-9a-f]{64})`)
)

// pushCharts builds, with Helm, the chart that each entry of the repository
// describes, pushes them to a new path of the registry that asks for no
// password, and points each entry at what was pushed. It returns the registry
// and path to give to index, and what was pushed.
func pushCharts(t *testing.T, repo string) (string, map[chartRef]pushedChart) {
	t.Helper()

	address := harness.OpenRegistry(t)
	registry := fmt.Sprintf("%s/e2e/%d", address, time.Now().UnixNano())

	pushed := map[chartRef]pushedChart{}
	entries, err := filepath.Glob(filepath.Join(repo, "entries", "*", "*.yaml"))
	require.NoError(t, err)
	for _, entry := range entries {
		ref := chartRef{chart: filepath.Base(filepath.Dir(entry)), version: strings.TrimSuffix(filepath.Base(entry), ".yaml")}
		pushed[ref] = pushChart(t, registry, entry, ref)
	}
	return registry, pushed
}

// pushChart makes the chart of one entry, as it was before ship changed it: the
// metadata of the entry, with the icon on the web, and no URL. It pushes it and
// rewrites the entry to point at the manifest that was pushed.
func pushChart(t *testing.T, registry, entry string, ref chartRef) pushedChart {
	t.Helper()

	text, err := os.ReadFile(entry)
	require.NoError(t, err)
	chartYAML := urlLine.ReplaceAllString(string(text), "")
	chartYAML = strings.Replace(chartYAML, "icon: file://assets/logos/", "icon: https://charts.rancher.io/assets/logos/", 1)

	work := t.TempDir()
	chartDir := filepath.Join(work, ref.chart)
	require.NoError(t, os.Mkdir(chartDir, 0o750))
	chartFile := filepath.Join(chartDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte(chartYAML), 0o600))
	require.NoError(t, os.Chtimes(chartFile, fixedTime(), fixedTime()))
	require.NoError(t, os.Chtimes(chartDir, fixedTime(), fixedTime()))

	helmEnv := []string{
		"HELM_REGISTRY_CONFIG=" + filepath.Join(work, "registry.json"),
		"HELM_REPOSITORY_CONFIG=" + filepath.Join(work, "repositories.yaml"),
		"HELM_REPOSITORY_CACHE=" + filepath.Join(work, "cache"),
	}
	res := harness.HelmWith(t, harness.Options{Dir: work, Env: helmEnv}, "package", chartDir, "--destination", work)
	require.Zero(t, res.ExitCode, res.Stderr)

	archive := filepath.Join(work, ref.chart+"-"+ref.version+".tgz")
	data, err := os.ReadFile(archive)
	require.NoError(t, err)

	// registry is address/path, and helm adds the name of the chart.
	address, path, _ := strings.Cut(registry, "/")
	res = harness.HelmWith(t, harness.Options{Dir: work, Env: helmEnv}, "push", archive, "oci://"+address+"/"+path, "--plain-http")
	require.Zero(t, res.ExitCode, res.Stderr)
	match := manifestDigest.FindStringSubmatch(res.Stdout + res.Stderr)
	require.NotNil(t, match, "helm push did not print the digest: %s%s", res.Stdout, res.Stderr)

	url := fmt.Sprintf("oci://%s/%s:%s@%s", registry, ref.chart, ref.version, match[1])
	require.NoError(t, os.WriteFile(entry, []byte(urlLine.ReplaceAllString(string(text), "urls:\n- "+url+"\n")), 0o600))
	return pushedChart{archiveDigest: digestOfBytes(data), url: url}
}

// expectedIndex turns the expected index of the fixture into the one that
// pulling the charts gives: each version has the digest of its archive and the
// URL it was pushed to.
func expectedIndex(t *testing.T, golden string, pushed map[chartRef]pushedChart) string {
	t.Helper()

	for ref, p := range pushed {
		block := regexp.MustCompile(`(?m)(    description: [^\n]*\n)((?:    [^\n]*\n)*?    urls:\n    - )oci://[^\n]*/` +
			regexp.QuoteMeta(ref.chart) + `:` + regexp.QuoteMeta(ref.version) + `@sha256:[0-9a-f]{64}\n`)
		require.Regexp(t, block, golden, "%s %s is in the expected index", ref.chart, ref.version)
		golden = block.ReplaceAllString(golden, "${1}    digest: "+p.archiveDigest+"\n${2}"+p.url+"\n")
	}
	return golden
}

func digestOfBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// runIndex runs index on the repository, pulling from registry.
func runIndex(t *testing.T, repo, registry string) harness.Result {
	t.Helper()

	return harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo, "--registry", registry, "--plain-http")
}

func TestIndexPullsTheNewVersions(t *testing.T) {
	repo := copyRepo(t)
	registry, pushed := pushCharts(t, repo)

	res := runIndex(t, repo, registry)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, allAdded(), res.Stdout)
	require.Equal(t, expectedIndex(t, string(mustRead(t, filepath.Join(fixture(t), "index.yaml"))), pushed), readIndex(t, repo))
	for ref, p := range pushed {
		got := mustRead(t, filepath.Join(repo, "assets", ref.chart, ref.chart+"-"+ref.version+".tgz"))
		require.Equal(t, p.archiveDigest, digestOfBytes(got), "%s %s", ref.chart, ref.version)
	}
}

func TestIndexPullsEachVersionOnce(t *testing.T) {
	repo := copyRepo(t)
	registry, _ := pushCharts(t, repo)
	res := runIndex(t, repo, registry)
	require.Zero(t, res.ExitCode, res.Stderr)
	first := readIndex(t, repo)

	// Nothing is new now, so nothing is pulled: a registry that does not exist
	// does no harm.
	res = runIndex(t, repo, "127.0.0.1:1/nothing")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, first, readIndex(t, repo))
}

func TestIndexPullsOnlyTheNewVersion(t *testing.T) {
	repo := copyRepo(t)
	registry, _ := pushCharts(t, repo)
	res := runIndex(t, repo, registry)
	require.Zero(t, res.ExitCode, res.Stderr)
	oldArchive := filepath.Join(repo, "assets", "fleet", "fleet-111.0.3+up0.17.3.tgz")
	before := mustRead(t, oldArchive)

	newEntry := filepath.Join(repo, "entries", "fleet", "111.0.5+up0.17.5.yaml")
	text := string(mustRead(t, filepath.Join(repo, "entries", "fleet", "111.0.3+up0.17.3.yaml")))
	text = strings.ReplaceAll(strings.ReplaceAll(text, "111.0.3+up0.17.3", "111.0.5+up0.17.5"), "appVersion: 0.17.3", "appVersion: 0.17.5")
	require.NoError(t, os.WriteFile(newEntry, []byte(text), 0o600))
	pushChart(t, registry, newEntry, chartRef{chart: "fleet", version: "111.0.5+up0.17.5"})

	res = runIndex(t, repo, registry)

	require.Zero(t, res.ExitCode, res.Stderr)
	require.FileExists(t, filepath.Join(repo, "assets", "fleet", "fleet-111.0.5+up0.17.5.tgz"))
	require.Contains(t, readIndex(t, repo), "version: 111.0.5+up0.17.5\n")
	require.Equal(t, before, mustRead(t, oldArchive), "a published archive is not touched")
}

func TestIndexChecksTheUrlAgainstTheRegistryGiven(t *testing.T) {
	repo := copyRepo(t)
	registry, _ := pushCharts(t, repo)

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo, "--plain-http")
	require.NotZero(t, res.ExitCode, "the staging registry is expected by default")
	require.Contains(t, res.Stderr, "but it must be oci://stgregistry.suse.com/rancher/catalog/")
	require.NoFileExists(t, filepath.Join(repo, "index.yaml"))

	res = runIndex(t, repo, registry)
	require.Zero(t, res.ExitCode, res.Stderr)
}

func TestIndexNeedsNoLogoForAChartThatIsExempt(t *testing.T) {
	repo := copyRepo(t)
	registry, _ := pushCharts(t, repo)
	// fleet is marked icon: skip, and fleet-crd is a crd chart.
	require.NoError(t, os.Remove(filepath.Join(repo, "assets", "logos", "fleet.svg")))

	res := runIndex(t, repo, registry)

	require.Zero(t, res.ExitCode, res.Stderr)
}

func TestIndexWritesNothingWhenAVersionCannotBePulled(t *testing.T) {
	repo := copyRepo(t)
	registry, _ := pushCharts(t, repo)
	// This manifest is not in the registry.
	editFile(t, filepath.Join(repo, "entries", "fleet", "111.0.2+up0.17.2.yaml"), func(s string) string {
		return pinnedDigest.ReplaceAllString(s, "@sha256:"+strings.Repeat("0", 64))
	})

	res := runIndex(t, repo, registry)

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "fleet 111.0.2+up0.17.2: pull the archive:")
	require.NoFileExists(t, filepath.Join(repo, "index.yaml"))
	require.NoDirExists(t, filepath.Join(repo, "assets", "fleet"), "the versions that could be pulled are not kept either")
}

func TestIndexRefusesAnArchiveThatIsNotTheEntry(t *testing.T) {
	repo := copyRepo(t)
	registry, _ := pushCharts(t, repo)
	// The entry says something else than the chart that was pushed.
	editFile(t, filepath.Join(repo, "entries", "fleet", "111.0.2+up0.17.2.yaml"), func(s string) string {
		return strings.Replace(s, "description: Fixture of fleet\n", "description: Changed after the push\n", 1)
	})

	res := runIndex(t, repo, registry)

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "fleet 111.0.2+up0.17.2: the Chart.yaml in the archive differs from the entry in: description")
	require.NoFileExists(t, filepath.Join(repo, "index.yaml"))
	require.NoDirExists(t, filepath.Join(repo, "assets", "fleet"))
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

// allAdded is what index prints when every version of the fixture is new.
func allAdded() string {
	return `add fleet 111.0.2+up0.17.2
add fleet 111.0.3+up0.17.3
add fleet 111.0.3+up0.17.4
add fleet 111.0.4+up0.17.4-rc.1
add fleet 111.0.4+up0.17.4-rc.10
add fleet 111.0.4+up0.17.4-rc.2
add fleet-crd 111.0.2+up0.17.2
add fleet-crd 111.0.3+up0.17.3
add longhorn 111.0.0+up1.10.0
add longhorn 111.0.1+up1.10.1
add system-upgrade-controller 111.0.0
add system-upgrade-controller 111.0.1
`
}

func TestIndexDryRunSaysWhatItWouldDoAndChangesNothing(t *testing.T) {
	repo := copyRepo(t)
	// The registry is never reached: none of these digests is in a registry.

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo, "--dry-run")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, allAdded(), res.Stdout)
	require.NoFileExists(t, filepath.Join(repo, "index.yaml"))
	require.NoDirExists(t, filepath.Join(repo, "assets", "fleet"))
	require.NoDirExists(t, filepath.Join(repo, "assets", "longhorn"))
}

func TestIndexDryRunStillChecks(t *testing.T) {
	repo := copyRepo(t)
	editFile(t, filepath.Join(repo, "entries", "fleet", "111.0.2+up0.17.2.yaml"), func(s string) string {
		return pinnedDigest.ReplaceAllString(s, "")
	})

	res := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo, "--dry-run")

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "fleet 111.0.2+up0.17.2 has the url")
	require.Empty(t, res.Stdout)
}

func TestIndexSaysThereIsNothingToChange(t *testing.T) {
	repo := publishedRepo(t)

	for _, args := range [][]string{{"--dry-run"}, {}} {
		res := harness.Cargueiro(t, t.TempDir(), append([]string{"index", "--repo", repo}, args...)...)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Equal(t, "nothing to change\n", res.Stdout)
	}
}
