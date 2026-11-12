package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/oci"
)

// runStampFamilyWith executes stamp with the fleet family of three charts in the
// rules and the registry given.
func runStampFamilyWith(t *testing.T, fake *fakeLister, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(familyRules()),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{listTags: fake.list})
	require.NoError(t, err)
	root.SetArgs(append([]string{"stamp"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

// runStampFamily is runStampFamilyWith with a registry that must not be asked.
func runStampFamily(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	return runStampFamilyWith(t, &fakeLister{err: errors.New("the registry must not be asked")}, args...)
}

// stampFamilyDirs makes a folder for each chart of the family: the main chart is
// a chart to stamp, the others only have to be there.
func stampFamilyDirs(t *testing.T) (fleet, agent, crd string) {
	t.Helper()

	fleet = writeChartDir(t, stampChartYAML, stampValuesYAML)
	agent = writeChartDir(t, familyChartYAML("fleet-agent", "0.16.3"), "")
	crd = writeChartDir(t, familyChartYAML("fleet-crd", "0.16.3"), "")
	return fleet, agent, crd
}

func readChartFile(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	return string(data)
}

func TestStampFamilyStampsTheMainChart(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	agentBefore, crdBefore := readChartFile(t, agent), readChartFile(t, crd)

	stdout, stderr, err := runStampFamily(t, "--family", "fleet", crd, agent, fleet, "--version", "111.0.0+up0.16.3")

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "==> fleet\nversion: 0.16.3 -> 111.0.0+up0.16.3\nhelm.sh/images: added fleet-agent, fleet\n", stdout)
	require.Contains(t, readChartFile(t, fleet), "version: 111.0.0+up0.16.3")
	require.Equal(t, agentBefore, readChartFile(t, agent), "the other charts are not changed")
	require.Equal(t, crdBefore, readChartFile(t, crd))
}

func TestStampFamilyTwice(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	args := []string{"--family", "fleet", fleet, agent, crd, "--version", "111.0.0+up0.16.3"}
	_, _, err := runStampFamily(t, args...)
	require.NoError(t, err)

	stdout, _, err := runStampFamily(t, args...)

	require.NoError(t, err)
	require.Equal(t, "==> fleet\nnothing to change\n", stdout)
}

func TestStampFamilyWorksOutTheVersionFromTheMainChart(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	fake := &fakeLister{tags: []string{"111.0.0_up0.16.1", "111.0.1_up0.16.2"}}

	stdout, _, err := runStampFamilyWith(t, fake, "--family", "fleet", fleet, agent, crd)

	require.NoError(t, err)
	require.Equal(t, "==> fleet\nversion: 0.16.3 -> 111.0.2+up0.16.3\nhelm.sh/images: added fleet-agent, fleet\n", stdout)
	require.Equal(t, oci.ChartRepo{Host: "stgregistry.suse.com", Repository: "rancher/catalog/fleet"}, fake.repo,
		"the registry of the main chart is the one that is read")
}

func TestStampFamilyDryRunWritesNothing(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)

	stdout, stderr, err := runStampFamily(t, "--family", "fleet", fleet, agent, crd, "--version", "111.0.0+up0.16.3", "--dry-run")

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.True(t, strings.HasPrefix(stdout, "==> fleet\n--- Chart.yaml\n+++ Chart.yaml\n"), stdout)
	require.Contains(t, stdout, "+version: 111.0.0+up0.16.3\n")
	require.Equal(t, stampChartYAML, readChartFile(t, fleet), "a dry run writes nothing")
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
