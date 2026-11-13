package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/rules"
)

// loadRules parses the rules file of the repository, as the binary embeds it.
func loadRules(t *testing.T) *rules.Config {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "rules.yaml"))
	require.NoError(t, err)
	cfg, err := rules.Parse(data)
	require.NoError(t, err)
	return cfg
}

// writeIndex writes an index.yaml to a new repository folder and returns it.
func writeIndex(t *testing.T, text string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.yaml"), []byte(text), 0o600))
	return dir
}

// fleetLine is a line of the index of the fleet chart, with the url given.
func fleetLine(version, url string) string {
	return "apiVersion: v1\nentries:\n  fleet:\n  - apiVersion: v2\n    name: fleet\n    version: " + version + "\n    urls:\n    - " + url + "\n"
}

const goodURL = "oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestRunReportsEveryCheck(t *testing.T) {
	dir := writeIndex(t, fleetLine("111.0.3+up0.17.3", goodURL))

	report := Run(context.Background(), loadRules(t), Options{Dir: dir})

	require.Len(t, report, 1)
	require.Equal(t, "urls", report[0].Check)
	require.True(t, report[0].Valid)
	require.Equal(t, NoError, report[0].Error)
	require.NotEmpty(t, report[0].Description)
	require.True(t, report.Valid())
}

func TestRunWithoutAnIndexHasNothingToCheck(t *testing.T) {
	for name, dir := range map[string]string{
		"no index.yaml":    t.TempDir(),
		"an empty file":    writeIndex(t, ""),
		"an index of none": writeIndex(t, "apiVersion: v1\nentries: {}\n"),
	} {
		t.Run(name, func(t *testing.T) {
			report := Run(context.Background(), loadRules(t), Options{Dir: dir})

			require.True(t, report.Valid())
			require.Contains(t, report[0].Description, "0 versions")
		})
	}
}

func TestRunSaysWhyACheckDidNotRun(t *testing.T) {
	dir := writeIndex(t, "this: is: not: yaml: [")

	report := Run(context.Background(), loadRules(t), Options{Dir: dir})

	require.Len(t, report, 1)
	require.False(t, report[0].Valid)
	require.Contains(t, report[0].Error, "read index.yaml")
	require.Contains(t, report[0].Description, "did not run")
	require.False(t, report.Valid())
}

func TestReportIsNotValidWhenOneCheckIsNot(t *testing.T) {
	report := Report{{Check: "a", Valid: true}, {Check: "b"}}

	require.False(t, report.Valid())
	require.True(t, Report{{Check: "a", Valid: true}}.Valid())
	require.True(t, Report{}.Valid())
}
