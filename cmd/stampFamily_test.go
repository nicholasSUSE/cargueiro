package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/oci"
)

// stampAgentYAML is the extra chart of the family: it is for Rancher 2.16, like
// the main chart, so it takes the same major.
const stampAgentYAML = `apiVersion: v2
name: fleet-agent
version: 0.16.3
appVersion: 0.16.3
annotations:
  catalog.cattle.io/certified: rancher
  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'
`

// repoLister answers with the tags of each repository, such as
// rancher/catalog/fleet, says that a repository it does not have is not found,
// and records the repositories it was asked about.
type repoLister struct {
	tags  map[string][]string
	asked []string
}

func (r *repoLister) list(_ context.Context, repo oci.ChartRepo, _ oci.FetchOptions) ([]string, error) {
	r.asked = append(r.asked, repo.Repository)
	tags, ok := r.tags[repo.Repository]
	if !ok {
		return nil, fmt.Errorf("%s was not found in the registry: %w", repo, oci.ErrRepositoryNotFound)
	}
	return tags, nil
}

// runStampFamilyWith executes stamp with the fleet family of three charts in the
// rules and the registry given.
func runStampFamilyWith(t *testing.T, list listFunc, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(familyRules()),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{listTags: list})
	require.NoError(t, err)
	root.SetArgs(append([]string{"stamp"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

// runStampFamily is runStampFamilyWith with a registry that must not be asked.
func runStampFamily(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	return runStampFamilyWith(t, (&fakeLister{err: errors.New("the registry must not be asked")}).list, args...)
}

// stampFamilyDirs makes a folder for each chart of the family.
func stampFamilyDirs(t *testing.T) (fleet, agent, crd string) {
	t.Helper()

	fleet = writeChartDir(t, stampChartYAML, stampValuesYAML)
	agent = writeChartDir(t, stampAgentYAML, "")
	crd = writeChartDir(t, familyChartYAML("fleet-crd", "0.16.3"), "")
	return fleet, agent, crd
}

func readChartFile(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	return string(data)
}

func TestStampFamilyStampsEveryChart(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)

	stdout, stderr, err := runStampFamily(t, "--family", "fleet", crd, agent, fleet, "--version", "111.0.0+up0.16.3")

	require.NoError(t, err)
	require.Empty(t, stderr)
	// The order of the family: the main chart first, then the order of the rules,
	// which list the CRD chart before the extra one.
	require.Equal(t, "==> fleet\nversion: 0.16.3 -> 111.0.0+up0.16.3\nhelm.sh/images: added fleet-agent, fleet\n"+
		"==> fleet-crd\nversion: 0.16.3 -> 111.0.0+up0.16.3\n"+
		"==> fleet-agent\nversion: 0.16.3 -> 111.0.0+up0.16.3\n", stdout)
	for _, dir := range []string{fleet, agent, crd} {
		require.Contains(t, readChartFile(t, dir), "\nversion: 111.0.0+up0.16.3\n")
	}
}

func TestStampFamilyTwice(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	args := []string{"--family", "fleet", fleet, agent, crd, "--version", "111.0.0+up0.16.3"}
	_, _, err := runStampFamily(t, args...)
	require.NoError(t, err)

	stdout, _, err := runStampFamily(t, args...)

	require.NoError(t, err)
	require.Equal(t, "==> fleet\nnothing to change\n==> fleet-crd\nnothing to change\n==> fleet-agent\nnothing to change\n", stdout)
}

func TestStampFamilyWorksOutTheVersionFromTheMainChartAndReadsEveryRegistry(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	same := []string{"111.0.0_up0.16.1", "111.0.1_up0.16.2"}
	registry := &repoLister{tags: map[string][]string{
		"rancher/catalog/fleet":       same,
		"rancher/catalog/fleet-agent": same,
		"rancher/catalog/fleet-crd":   same,
	}}

	stdout, _, err := runStampFamilyWith(t, registry.list, "--family", "fleet", fleet, agent, crd)

	require.NoError(t, err)
	require.Contains(t, stdout, "==> fleet\nversion: 0.16.3 -> 111.0.2+up0.16.3\n")
	require.Contains(t, stdout, "==> fleet-crd\nversion: 0.16.3 -> 111.0.2+up0.16.3\n")
	require.Equal(t, []string{"rancher/catalog/fleet", "rancher/catalog/fleet-crd", "rancher/catalog/fleet-agent"}, registry.asked,
		"one read for each registry")
}

func TestStampFamilyRefusesWhenTheLatestVersionsDiffer(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	registry := &repoLister{tags: map[string][]string{
		"rancher/catalog/fleet":       {"111.0.0_up0.16.1", "111.0.1_up0.16.2"},
		"rancher/catalog/fleet-agent": {"111.0.0_up0.16.1", "111.0.1_up0.16.2"},
		"rancher/catalog/fleet-crd":   {"111.0.0_up0.16.1"},
	}}

	stdout, _, err := runStampFamilyWith(t, registry.list, "--family", "fleet", fleet, agent, crd)

	require.ErrorContains(t, err, `the charts of the family "fleet" are not at the same released version: `+
		"fleet 111.0.1+up0.16.2, fleet-crd 111.0.0+up0.16.1, fleet-agent 111.0.1+up0.16.2: give --version to set it by hand")
	require.Empty(t, stdout)
	require.Equal(t, stampChartYAML, readChartFile(t, fleet), "nothing is changed")
	require.Equal(t, stampAgentYAML, readChartFile(t, agent))
}

func TestStampFamilyDryRunWritesNothing(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)

	stdout, stderr, err := runStampFamily(t, "--family", "fleet", fleet, agent, crd, "--version", "111.0.0+up0.16.3", "--dry-run")

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.True(t, strings.HasPrefix(stdout, "==> fleet\n--- Chart.yaml\n+++ Chart.yaml\n"), stdout)
	require.Contains(t, stdout, "==> fleet-agent\n--- Chart.yaml\n")
	require.Contains(t, stdout, "==> fleet-crd\n--- Chart.yaml\n")
	require.Equal(t, stampChartYAML, readChartFile(t, fleet), "a dry run writes nothing")
	require.Equal(t, stampAgentYAML, readChartFile(t, agent))
}

func TestStampFamilyChangesNothingWhenOneChartCannotTakeTheVersion(t *testing.T) {
	fleet, _, crd := stampFamilyDirs(t)
	// The extra chart is for Rancher 2.17, whose charts have the major 112.
	agentFor217 := strings.Replace(stampAgentYAML, ">= 2.16.0-0 < 2.17.0-0", ">= 2.17.0-0 < 2.18.0-0", 1)
	agent := writeChartDir(t, agentFor217, "")

	stdout, _, err := runStampFamily(t, "--family", "fleet", fleet, agent, crd, "--version", "111.0.0+up0.16.3")

	require.ErrorContains(t, err, "fleet-agent: ")
	require.Empty(t, stdout)
	require.Equal(t, stampChartYAML, readChartFile(t, fleet), "the charts that could take it are not changed either")
	require.Equal(t, agentFor217, readChartFile(t, agent))
}

func TestStampFamilyRefusesAndChangesNothing(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	longhorn := writeChartDir(t, familyChartYAML("longhorn", "0.16.3"), "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "a chart is missing",
			args:    []string{"--family", "fleet", fleet, crd, "--version", "111.0.0+up0.16.3"},
			wantErr: `the family "fleet" is missing fleet-agent`,
		},
		{
			name:    "a chart is given twice",
			args:    []string{"--family", "fleet", fleet, fleet, agent, crd, "--version", "111.0.0+up0.16.3"},
			wantErr: `chart "fleet" is given twice`,
		},
		{
			name:    "a chart of another family",
			args:    []string{"--family", "fleet", fleet, agent, crd, longhorn, "--version", "111.0.0+up0.16.3"},
			wantErr: `chart "longhorn" is not in the family "fleet"`,
		},
		{
			name:    "a family that is not listed",
			args:    []string{"--family", "nope", fleet, "--version", "111.0.0+up0.16.3"},
			wantErr: `family "nope" is not listed in the rules file`,
		},
		{
			name:    "no folders",
			args:    []string{"--family", "fleet", "--version", "111.0.0+up0.16.3"},
			wantErr: "requires at least 1 arg(s)",
		},
		{
			name:    "an empty version",
			args:    []string{"--family", "fleet", fleet, agent, crd, "--version", ""},
			wantErr: "--version is empty",
		},
		{
			name:    "a version the main chart cannot have",
			args:    []string{"--family", "fleet", fleet, agent, crd, "--version", "112.0.0+up0.16.3"},
			wantErr: "fleet: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runStampFamily(t, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.Equal(t, stampChartYAML, readChartFile(t, fleet), "nothing is changed")
		})
	}
}

func TestStampWithoutAFamilyStillTakesOneFolder(t *testing.T) {
	fleet, agent, _ := stampFamilyDirs(t)

	_, _, err := runStampFamily(t, fleet, agent, "--version", "111.0.0+up0.16.3")

	require.ErrorContains(t, err, "accepts 1 arg(s), received 2")
}
