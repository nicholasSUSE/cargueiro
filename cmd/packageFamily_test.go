package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/chart"
)

const packageFamilyVersion = "111.0.0+up0.16.3"

// runPackageFamily executes package with the fleet family of three charts in the
// rules.
func runPackageFamily(t *testing.T, packager *countingPackager, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(familyRules()),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{pkg: packager.pkg})
	require.NoError(t, err)
	root.SetArgs(append([]string{"package"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

// packageFamilyDirs makes a stamped folder for each chart of the family.
func packageFamilyDirs(t *testing.T) (fleet, agent, crd string) {
	t.Helper()

	template := map[string]string{"templates/deployment.yaml": "kind: Deployment\n"}
	fleet = writeChartDir(t, stampedChartYAML, stampedValuesYAML)
	agent = writeChartDir(t, familyChartYAML("fleet-agent", packageFamilyVersion), "")
	crd = writeChartDir(t, familyChartYAML("fleet-crd", packageFamilyVersion), "")
	for _, dir := range []string{fleet, agent, crd} {
		writeFiles(t, dir, template)
	}
	return fleet, agent, crd
}

func archiveOf(out, name string) string {
	return filepath.Join(out, name+"-"+packageFamilyVersion+".tgz")
}

func TestPackageFamilyPackagesEveryChart(t *testing.T) {
	fleet, agent, crd := packageFamilyDirs(t)
	out := filepath.Join(t.TempDir(), "out")
	packager := &countingPackager{}

	stdout, stderr, err := runPackageFamily(t, packager, "--family", "fleet", crd, agent, fleet, "-o", out)

	require.NoError(t, err)
	require.Empty(t, stderr)
	// The order of the family: the main chart first, then the order of the rules,
	// which list the CRD chart before the extra one.
	require.Equal(t, archiveOf(out, "fleet")+"\n"+archiveOf(out, "fleet-crd")+"\n"+archiveOf(out, "fleet-agent")+"\n", stdout)
	for _, name := range []string{"fleet", "fleet-crd", "fleet-agent"} {
		require.FileExists(t, archiveOf(out, name))
	}
	require.Equal(t, 3, packager.calls)
}

func TestPackageFamilyBuildsNothingWhenAChartIsNotValid(t *testing.T) {
	fleet, _, crd := packageFamilyDirs(t)
	agent := writeChartDir(t, "name: fleet-agent\nversion: "+packageFamilyVersion+"\n", "")
	out := filepath.Join(t.TempDir(), "out")
	packager := &countingPackager{}

	stdout, stderr, err := runPackageFamily(t, packager, "--family", "fleet", fleet, agent, crd, "-o", out)

	require.ErrorContains(t, err, "fleet-agent: the chart is not valid, so the family was not packaged")
	require.Empty(t, stdout)
	require.Contains(t, stderr, "chart: fleet-agent")
	require.NotContains(t, stderr, "chart: fleet-crd", "only the report of a chart that is not valid is written")
	require.Zero(t, packager.calls, "no archive is built")
	require.NoDirExists(t, out)
}

func TestPackageFamilyBuildsNothingWhenTheVersionsDiffer(t *testing.T) {
	fleet, _, crd := packageFamilyDirs(t)
	agent := writeChartDir(t, familyChartYAML("fleet-agent", "111.0.1+up0.16.3"), "")
	out := filepath.Join(t.TempDir(), "out")
	packager := &countingPackager{}

	stdout, _, err := runPackageFamily(t, packager, "--family", "fleet", fleet, agent, crd, "-o", out)

	require.ErrorContains(t, err, `the charts of the family "fleet" do not have the same version`)
	require.Empty(t, stdout)
	require.Zero(t, packager.calls)
	require.NoDirExists(t, out)
}

func TestPackageFamilyRemovesTheArchivesWhenAChartFails(t *testing.T) {
	fleet, agent, crd := packageFamilyDirs(t)
	out := filepath.Join(t.TempDir(), "out")
	packager := &countingPackager{pack: func(ctx context.Context, src, dest string) (string, error) {
		if src == agent {
			return "", errors.New("the disk is full")
		}
		return chart.Package(ctx, src, dest)
	}}

	stdout, _, err := runPackageFamily(t, packager, "--family", "fleet", fleet, agent, crd, "-o", out)

	require.ErrorContains(t, err, "fleet-agent: the disk is full")
	require.Empty(t, stdout)
	require.Equal(t, 3, packager.calls, "the main chart and the CRD chart were built before the extra one failed")
	require.NoDirExists(t, out, "the folder was made for them and is gone with them")
}

func TestPackageFamilyKeepsAFolderThatWasThereButRemovesItsArchives(t *testing.T) {
	fleet, agent, crd := packageFamilyDirs(t)
	out := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(out, "other.txt"), []byte("mine"), 0o600))
	packager := &countingPackager{pack: func(ctx context.Context, src, dest string) (string, error) {
		if src == agent {
			return "", errors.New("the disk is full")
		}
		return chart.Package(ctx, src, dest)
	}}

	_, _, err := runPackageFamily(t, packager, "--family", "fleet", fleet, agent, crd, "-o", out)

	require.Error(t, err)
	require.FileExists(t, filepath.Join(out, "other.txt"), "what was in the folder stays")
	entries, readErr := os.ReadDir(out)
	require.NoError(t, readErr)
	require.Len(t, entries, 1, "and the archives that were built are gone")
}

func TestPackageFamilyRemovesTheArchivesWhenOneIsNotValid(t *testing.T) {
	fleet, agent, crd := packageFamilyDirs(t)
	notStamped := writeChartDir(t, stampChartYAML, stampValuesYAML)
	out := filepath.Join(t.TempDir(), "out")
	packager := &countingPackager{pack: func(ctx context.Context, src, dest string) (string, error) {
		if src == crd {
			// An archive of a chart that was not stamped, instead of the one of the CRD chart.
			return chart.Package(ctx, notStamped, dest)
		}
		return chart.Package(ctx, src, dest)
	}}

	stdout, _, err := runPackageFamily(t, packager, "--family", "fleet", fleet, agent, crd, "-o", out)

	require.ErrorContains(t, err, "fleet-crd: the archive is not valid, so it was removed")
	require.Empty(t, stdout)
	require.NoDirExists(t, out, "the archive of the main chart is gone too")
}

func TestPackageFamilyRefuses(t *testing.T) {
	fleet, agent, crd := packageFamilyDirs(t)
	longhorn := writeChartDir(t, familyChartYAML("longhorn", packageFamilyVersion), "")

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
			name:    "a family that is not listed",
			args:    []string{"--family", "nope", fleet},
			wantErr: `family "nope" is not listed in the rules file`,
		},
		{
			name:    "no folders",
			args:    []string{"--family", "fleet"},
			wantErr: "requires at least 1 arg(s)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out")
			packager := &countingPackager{}

			stdout, _, err := runPackageFamily(t, packager, append(tt.args, "-o", out)...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.Zero(t, packager.calls)
			require.NoDirExists(t, out)
		})
	}
}

func TestPackageWithoutAFamilyStillTakesOneFolder(t *testing.T) {
	fleet, agent, _ := packageFamilyDirs(t)

	_, _, err := runPackageFamily(t, &countingPackager{}, fleet, agent)

	require.ErrorContains(t, err, "accepts 1 arg(s), received 2")
}
