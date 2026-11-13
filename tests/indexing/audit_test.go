//go:build e2e

package indexing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

// auditResult is a result of the JSON that audit prints.
type auditResult struct {
	Check       string `json:"check"`
	Valid       bool   `json:"valid"`
	Description string `json:"description"`
	Error       string `json:"error"`
}

// runAudit runs audit on the repository, reading registry, and returns the
// results by the name of their check.
func runAudit(t *testing.T, repo, registry string) (harness.Result, map[string]auditResult) {
	t.Helper()

	res := harness.Cargueiro(t, t.TempDir(), "audit", "--repo", repo, "--registry", registry, "--plain-http", "--output", "json")
	var results []auditResult
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &results), "stdout: %s stderr: %s", res.Stdout, res.Stderr)

	byName := map[string]auditResult{}
	for _, r := range results {
		byName[r.Check] = r
	}
	require.Len(t, byName, 3)
	return res, byName
}

// indexedRepo returns a repository whose charts were pushed to a new path of the
// registry, and whose index.yaml index wrote from them.
func indexedRepo(t *testing.T) (repo, registry string) {
	t.Helper()

	repo = copyRepo(t)
	registry, _ = pushCharts(t, repo)
	res := runIndex(t, repo, registry)
	require.Zero(t, res.ExitCode, res.Stderr)
	return repo, registry
}

func TestAuditPassesOnWhatIndexWrote(t *testing.T) {
	repo, registry := indexedRepo(t)

	res, results := runAudit(t, repo, registry)

	require.Zero(t, res.ExitCode, res.Stderr)
	for name, r := range results {
		require.True(t, r.Valid, "%s: %s", name, r.Error)
		require.Equal(t, "no-error", r.Error, name)
	}
}

func TestAuditFindsAnUrlThatPointsToAnotherPath(t *testing.T) {
	repo, registry := indexedRepo(t)
	path := filepath.Join(repo, "index.yaml")
	text := mustRead(t, path)
	changed := regexp.MustCompile(`(oci://[^/]+)/e2e/\d+/`).ReplaceAllString(string(text), "${1}/other/")
	require.NotEqual(t, string(text), changed)
	require.NoError(t, os.WriteFile(path, []byte(changed), 0o600))

	res, results := runAudit(t, repo, registry)

	require.Equal(t, 1, res.ExitCode)
	require.False(t, results["urls"].Valid)
	require.True(t, results["manifest"].Valid, "the registry is read where the rules say, not where the url says")
	require.True(t, results["archive"].Valid)
}

func TestAuditFindsADigestThatIsNotTheOneOfTheArchive(t *testing.T) {
	repo, registry := indexedRepo(t)
	path := filepath.Join(repo, "index.yaml")
	text := string(mustRead(t, path))
	digest := regexp.MustCompile(`(?m)^    digest: ([0-9a-f]{64})$`).FindStringSubmatch(text)
	require.NotNil(t, digest)
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(text, digest[1], strings.Repeat("0", 63)+"a", 1)), 0o600))

	res, results := runAudit(t, repo, registry)

	require.Equal(t, 1, res.ExitCode)
	require.False(t, results["archive"].Valid)
	require.Contains(t, results["archive"].Error, "the index has the digest sha256:"+strings.Repeat("0", 63)+"a")
	require.True(t, results["urls"].Valid)
	require.True(t, results["manifest"].Valid)
}

func TestAuditFindsATagPushedAgainWithOtherContent(t *testing.T) {
	repo, registry := indexedRepo(t)
	entry := filepath.Join(repo, "entries", "fleet", "111.0.3+up0.17.3.yaml")
	other := filepath.Join(t.TempDir(), filepath.Base(entry))
	text := regexp.MustCompile(`(?m)^description: .*$`).ReplaceAllString(string(mustRead(t, entry)), "description: Another chart under the same tag")
	require.NoError(t, os.WriteFile(other, []byte(text), 0o600))
	pushChart(t, registry, other, chartRef{chart: "fleet", version: "111.0.3+up0.17.3"})

	res, results := runAudit(t, repo, registry)

	require.Equal(t, 1, res.ExitCode)
	require.False(t, results["manifest"].Valid)
	require.Contains(t, results["manifest"].Error, "fleet 111.0.3+up0.17.3: the tag 111.0.3_up0.17.3")
	require.NotContains(t, results["manifest"].Error, "fleet-crd", "the other charts are right")
}

func TestAuditFindsVersionsThatAreNotInTheRegistry(t *testing.T) {
	repo, registry := indexedRepo(t)

	res, results := runAudit(t, repo, registry+"-empty")

	require.Equal(t, 1, res.ExitCode)
	require.False(t, results["urls"].Valid)
	require.False(t, results["manifest"].Valid)
	require.Contains(t, results["manifest"].Error, "was not found in the registry")
	require.True(t, results["archive"].Valid, "a version that is not there is left to the manifest check")
}
