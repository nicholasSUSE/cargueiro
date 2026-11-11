package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// familyChartYAML is the Chart.yaml of a chart of the fleet family.
func familyChartYAML(name, version string) string {
	return fmt.Sprintf(`
name: %s
version: %s
appVersion: 0.16.3
icon: https://example.com/icon.png
description: %s
annotations:
  catalog.cattle.io/certified: rancher
  catalog.cattle.io/kube-version: ">= 1.30.0-0 < 1.35.0-0"
`, name, version, name)
}

// notCertifiedYAML is a chart of the family without a required annotation.
func notCertifiedYAML(name string) string {
	return fmt.Sprintf("\nname: %s\nversion: %s\n", name, familyVersionForCheck)
}

const familyVersionForCheck = "111.0.3+up0.16.3"

// runCheckFamily executes check <what> with the fleet family of three charts in
// the rules.
func runCheckFamily(t *testing.T, what string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(familyRules()),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{})
	require.NoError(t, err)
	root.SetArgs(append([]string{"check", what}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

// fleetDirs makes a folder for each chart of the family and returns them in the
// order fleet-agent, fleet, fleet-crd, which is not the order of the family.
func fleetDirs(t *testing.T) (agent, fleet, crd string) {
	t.Helper()

	agent = writeChartDir(t, familyChartYAML("fleet-agent", familyVersionForCheck), "")
	fleet = writeChartDir(t, familyChartYAML("fleet", familyVersionForCheck), "")
	crd = writeChartDir(t, familyChartYAML("fleet-crd", familyVersionForCheck), "")
	return agent, fleet, crd
}

func TestCheckChartFamilyReportsEveryChartInTheFamilyOrder(t *testing.T) {
	agent, fleet, crd := fleetDirs(t)

	stdout, stderr, err := runCheckFamily(t, "chart", "--family", "fleet", agent, fleet, crd)

	require.NoError(t, err)
	require.Empty(t, stderr)
	var charts []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "chart: ") {
			charts = append(charts, strings.TrimPrefix(line, "chart: "))
		}
	}
	require.Equal(t, []string{"fleet", "fleet-crd", "fleet-agent"}, charts, "the main chart first, then the order of the rules")
	require.Equal(t, 2, strings.Count(stdout, "\n\nvalid: true"), "a blank line between the results")
}

func TestCheckChartFamilyAsJSONIsAList(t *testing.T) {
	agent, fleet, crd := fleetDirs(t)

	stdout, _, err := runCheckFamily(t, "chart", "--family", "fleet", "--output", "json", agent, fleet, crd)

	require.NoError(t, err)
	var results []struct {
		Valid bool   `json:"valid"`
		Chart string `json:"chart"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &results))
	require.Len(t, results, 3)
	for i, want := range []string{"fleet", "fleet-crd", "fleet-agent"} {
		require.Equal(t, want, results[i].Chart)
		require.True(t, results[i].Valid)
	}
}

func TestCheckChartFamilyReportsEveryChartWhenOneIsNotValid(t *testing.T) {
	agent := writeChartDir(t, notCertifiedYAML("fleet-agent"), "")
	fleet := writeChartDir(t, familyChartYAML("fleet", familyVersionForCheck), "")
	crd := writeChartDir(t, familyChartYAML("fleet-crd", familyVersionForCheck), "")

	stdout, _, err := runCheckFamily(t, "chart", "--family", "fleet", fleet, agent, crd)

	require.EqualError(t, err, `the family "fleet" is not valid: fleet-agent`)
	require.Contains(t, stdout, "chart: fleet\n")
	require.Contains(t, stdout, "chart: fleet-crd\n")
	require.Contains(t, stdout, "chart: fleet-agent\n", "the valid charts are reported too")
	require.Contains(t, stdout, "annotations: invalid")
}

func TestCheckChartFamilyFailsWhenTheVersionsDiffer(t *testing.T) {
	agent := writeChartDir(t, familyChartYAML("fleet-agent", "111.0.4+up0.16.4"), "")
	fleet := writeChartDir(t, familyChartYAML("fleet", familyVersionForCheck), "")
	crd := writeChartDir(t, familyChartYAML("fleet-crd", familyVersionForCheck), "")

	stdout, _, err := runCheckFamily(t, "chart", "--family", "fleet", fleet, agent, crd)

	require.ErrorContains(t, err, `the charts of the family "fleet" do not have the same version: fleet 111.0.3+up0.16.3, fleet-crd 111.0.3+up0.16.3, fleet-agent 111.0.4+up0.16.4`)
	require.Contains(t, stdout, "chart: fleet-agent\n", "every chart is still reported")
}

func TestCheckChartFamilyNoErrorHIP15(t *testing.T) {
	agent := writeChartDir(t, familyChartYAML("fleet-agent", familyVersionForCheck), imagesValuesYAML)
	fleet := writeChartDir(t, familyChartYAML("fleet", familyVersionForCheck), "")
	crd := writeChartDir(t, familyChartYAML("fleet-crd", familyVersionForCheck), "")

	t.Run("an image that is not listed fails", func(t *testing.T) {
		_, _, err := runCheckFamily(t, "chart", "--family", "fleet", fleet, agent, crd)

		require.EqualError(t, err, `the family "fleet" is not valid: fleet-agent`)
	})

	t.Run("and passes when only HIP-15 is ignored", func(t *testing.T) {
		stdout, stderr, err := runCheckFamily(t, "chart", "--family", "fleet", "--no-error-hip-15", fleet, agent, crd)

		require.NoError(t, err)
		require.Contains(t, stdout, "hip15-annotations: invalid")
		require.Equal(t, "chart is not valid (HIP-15 errors ignored by --no-error-hip-15)\n", stderr)
	})
}

func TestCheckChartFamilyRefuses(t *testing.T) {
	agent, fleet, crd := fleetDirs(t)
	longhorn := writeChartDir(t, familyChartYAML("longhorn", familyVersionForCheck), "")
	unreadable := t.TempDir()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "a chart is missing",
			args:    []string{"--family", "fleet", fleet, crd},
			wantErr: `the family "fleet" is missing fleet-agent`,
		},
		{
			name:    "a chart is given twice",
			args:    []string{"--family", "fleet", fleet, fleet, agent, crd},
			wantErr: `chart "fleet" is given twice`,
		},
		{
			name:    "a chart of another family",
			args:    []string{"--family", "fleet", fleet, agent, crd, longhorn},
			wantErr: `chart "longhorn" is not in the family "fleet"`,
		},
		{
			name:    "a folder that cannot be read",
			args:    []string{"--family", "fleet", fleet, agent, unreadable},
			wantErr: "Chart.yaml",
		},
		{
			name:    "no folders",
			args:    []string{"--family", "fleet"},
			wantErr: "requires at least 1 arg(s)",
		},
		{
			name:    "a family that is not listed",
			args:    []string{"--family", "nope", fleet},
			wantErr: `family "nope" is not listed in the rules file`,
		},
		{
			name:    "an output that is not known",
			args:    []string{"--family", "fleet", "--output", "yaml", fleet, agent, crd},
			wantErr: `unknown output format "yaml"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runCheckFamily(t, "chart", tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout, "nothing is printed when the set is not the family")
		})
	}
}

func TestCheckChartWithoutAFamilyStillTakesOneFolder(t *testing.T) {
	agent, fleet, _ := fleetDirs(t)

	_, _, err := runCheckFamily(t, "chart", fleet, agent)

	require.ErrorContains(t, err, "accepts 1 arg(s), received 2")
}

func TestCheckArchiveFamily(t *testing.T) {
	agent := writeArchive(t, familyChartYAML("fleet-agent", familyVersionForCheck), "")
	fleet := writeArchive(t, familyChartYAML("fleet", familyVersionForCheck), "")
	crd := writeArchive(t, familyChartYAML("fleet-crd", familyVersionForCheck), "")

	t.Run("reports every archive in the family order", func(t *testing.T) {
		stdout, stderr, err := runCheckFamily(t, "archive", "--family", "fleet", agent, fleet, crd)

		require.NoError(t, err)
		require.Empty(t, stderr)
		first := strings.Index(stdout, "chart: fleet\n")
		second := strings.Index(stdout, "chart: fleet-crd\n")
		third := strings.Index(stdout, "chart: fleet-agent\n")
		require.True(t, first >= 0 && first < second && second < third, stdout)
	})

	t.Run("an archive that is missing", func(t *testing.T) {
		stdout, _, err := runCheckFamily(t, "archive", "--family", "fleet", fleet, crd)

		require.ErrorContains(t, err, `the family "fleet" is missing fleet-agent`)
		require.Empty(t, stdout)
	})

	t.Run("an archive that is not there", func(t *testing.T) {
		stdout, _, err := runCheckFamily(t, "archive", "--family", "fleet", fleet, crd, "/nope/fleet-agent.tgz")

		require.ErrorContains(t, err, "/nope/fleet-agent.tgz not found")
		require.Empty(t, stdout)
	})

	t.Run("one that is not valid fails the family", func(t *testing.T) {
		bad := writeArchive(t, notCertifiedYAML("fleet-agent"), "")

		_, _, err := runCheckFamily(t, "archive", "--family", "fleet", fleet, crd, bad)

		require.EqualError(t, err, `the family "fleet" is not valid: fleet-agent`)
	})
}
