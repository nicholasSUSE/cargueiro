package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/oci"
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

// nothingInRegistry is a registry that has no tag: tests that do not look at it
// use it, so none of them reaches a real one.
func nothingInRegistry(context.Context, oci.ChartRef, oci.FetchOptions) (oci.ManifestInfo, error) {
	return oci.ManifestInfo{}, fmt.Errorf("%w", oci.ErrNotFound)
}

// run audits dir with the rules of the repository, and a registry with nothing
// in it unless the options give one.
func run(t *testing.T, opts Options) Report {
	t.Helper()

	if opts.Fetch == nil {
		opts.Fetch = nothingInRegistry
	}
	return Run(context.Background(), loadRules(t), opts)
}

const goodURL = "oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestRunReportsEveryCheck(t *testing.T) {
	dir := writeIndex(t, fleetLine("111.0.3+up0.17.3", goodURL))

	report := run(t, Options{Dir: dir})

	require.Len(t, report, 2)
	require.Equal(t, "urls", report[0].Check)
	require.Equal(t, "manifest", report[1].Check)
	require.True(t, report[0].Valid)
	require.Equal(t, NoError, report[0].Error)
	for _, result := range report {
		require.NotEmpty(t, result.Description)
	}
}

func TestRunWithoutAnIndexHasNothingToCheck(t *testing.T) {
	for name, dir := range map[string]string{
		"no index.yaml":    t.TempDir(),
		"an empty file":    writeIndex(t, ""),
		"an index of none": writeIndex(t, "apiVersion: v1\nentries: {}\n"),
	} {
		t.Run(name, func(t *testing.T) {
			report := run(t, Options{Dir: dir})

			require.True(t, report.Valid(), report)
			require.Contains(t, report[0].Description, "0 versions")
		})
	}
}

func TestRunSaysWhyACheckDidNotRun(t *testing.T) {
	dir := writeIndex(t, "this: is: not: yaml: [")

	report := run(t, Options{Dir: dir})

	require.Len(t, report, 2)
	for _, result := range report {
		require.False(t, result.Valid)
		require.Contains(t, result.Error, "read index.yaml")
		require.Contains(t, result.Description, "did not run")
	}
	require.False(t, report.Valid())
}

func TestReportIsNotValidWhenOneCheckIsNot(t *testing.T) {
	report := Report{{Check: "a", Valid: true}, {Check: "b"}}

	require.False(t, report.Valid())
	require.True(t, Report{{Check: "a", Valid: true}}.Valid())
	require.True(t, Report{}.Valid())
}
