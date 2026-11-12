package cmd

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/oci"
)

// pushFamilyArchives packages the three charts of the family and returns the
// archives of the main chart, the extra one and the CRD chart.
func pushFamilyArchives(t *testing.T) (fleet, agent, crd string) {
	t.Helper()

	fleetDir, agentDir, crdDir := packageFamilyDirs(t)
	out := t.TempDir()
	pack := func(dir string) string {
		archive, err := chart.Package(context.Background(), dir, out)
		require.NoError(t, err)
		return archive
	}
	return pack(fleetDir), pack(agentDir), pack(crdDir)
}

// runPushFamily executes push with the fleet family of three charts in the
// rules, and a pusher factory that records whether it was used.
func runPushFamily(t *testing.T, pusher *fakePusher, args ...string) (stdout, stderr string, built bool, err error) {
	t.Helper()

	factory := func(oci.Target, *oci.Guard, oci.ClientOptions) (archivePusher, error) {
		built = true
		return pusher, nil
	}
	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(familyRules()),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{newPusher: factory})
	require.NoError(t, err)
	root.SetArgs(append([]string{"push"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), built, err
}

func member(chart string, state oci.MemberState, ref string) oci.MemberResult {
	return oci.MemberResult{Chart: chart, Version: packageFamilyVersion, State: state, Ref: ref}
}

func TestPushFamilyPushesInTheOrderOfTheFamilyAndPrintsTheReferences(t *testing.T) {
	fleet, agent, crd := pushFamilyArchives(t)
	pusher := &fakePusher{family: &oci.FamilyResult{Members: []oci.MemberResult{
		member("fleet", oci.StatePushed, "reg.local/charts/fleet:111.0.0_up0.16.3"),
		member("fleet-crd", oci.StatePushed, "reg.local/charts/fleet-crd:111.0.0_up0.16.3"),
		member("fleet-agent", oci.StatePushed, "reg.local/charts/fleet-agent:111.0.0_up0.16.3"),
	}}}

	stdout, stderr, _, err := runPushFamily(t, pusher, "--family", "fleet", agent, crd, fleet, "--registry", "reg.local/charts")

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, []string{fleet, crd, agent}, pusher.familyArchives, "the order of the family, whatever the order given")
	require.Equal(t, "oci://reg.local/charts/fleet:111.0.0_up0.16.3\n"+
		"oci://reg.local/charts/fleet-crd:111.0.0_up0.16.3\n"+
		"oci://reg.local/charts/fleet-agent:111.0.0_up0.16.3\n", stdout)
}

func TestPushFamilySaysWhichChartsWereThereAlready(t *testing.T) {
	fleet, agent, crd := pushFamilyArchives(t)
	pusher := &fakePusher{family: &oci.FamilyResult{Members: []oci.MemberResult{
		member("fleet", oci.StateSkipped, "reg.local/charts/fleet:111.0.0_up0.16.3"),
		member("fleet-crd", oci.StatePushed, "reg.local/charts/fleet-crd:111.0.0_up0.16.3"),
		member("fleet-agent", oci.StatePushed, "reg.local/charts/fleet-agent:111.0.0_up0.16.3"),
	}}}

	stdout, stderr, _, err := runPushFamily(t, pusher, "--family", "fleet", fleet, agent, crd, "--registry", "reg.local/charts")

	require.NoError(t, err)
	require.Equal(t, "warning: fleet 111.0.0+up0.16.3 is in the registry already, so it was not pushed\n", stderr)
	require.Contains(t, stdout, "oci://reg.local/charts/fleet:111.0.0_up0.16.3\n", "its reference is printed too")
}

func TestPushFamilyTellsWhenItTriesAgain(t *testing.T) {
	fleet, agent, crd := pushFamilyArchives(t)
	pusher := &fakePusher{
		family: &oci.FamilyResult{Members: []oci.MemberResult{member("fleet", oci.StatePushed, "r/fleet:1")}},
		beforeFamily: func(notice oci.RetryNotice) {
			notice("fleet-crd", 1, 5, errors.New("i/o timeout"), 30*time.Second)
		},
	}

	_, stderr, _, err := runPushFamily(t, pusher, "--family", "fleet", fleet, agent, crd, "--registry", "reg.local/charts")

	require.NoError(t, err)
	require.Contains(t, stderr, "warning: fleet-crd: attempt 1 of 5 failed (i/o timeout): trying again in 30s\n")
}

func TestPushFamilyReportsWhatHappenedToEveryChartWhenOneFails(t *testing.T) {
	fleet, agent, crd := pushFamilyArchives(t)
	failed := member("fleet-crd", oci.StateFailed, "")
	failed.Err = errors.New("i/o timeout")
	pusher := &fakePusher{
		family: &oci.FamilyResult{Members: []oci.MemberResult{
			member("fleet", oci.StatePushed, "reg.local/charts/fleet:111.0.0_up0.16.3"),
			failed,
			member("fleet-agent", oci.StateNotTried, ""),
		}},
		familyErr: errors.New("push reg.local/charts/fleet-crd:111.0.0+up0.16.3: i/o timeout"),
	}

	stdout, stderr, _, err := runPushFamily(t, pusher, "--family", "fleet", fleet, agent, crd, "--registry", "reg.local/charts")

	require.ErrorContains(t, err, "i/o timeout")
	require.ErrorContains(t, err, "run the command again to finish it")
	require.Empty(t, stdout, "no reference is printed unless every chart is in the registry")
	require.Contains(t, stderr, `the family "fleet" was not pushed completely:`)
	require.Contains(t, stderr, "  pushed:        fleet 111.0.0+up0.16.3\n")
	require.Contains(t, stderr, "  already there: -\n")
	require.Contains(t, stderr, "  failed:        fleet-crd 111.0.0+up0.16.3 (i/o timeout)\n")
	require.Contains(t, stderr, "  not tried:     fleet-agent 111.0.0+up0.16.3")
}

func TestPushFamilySaysNothingWasPushedWhenItFailsBeforeTheFirstUpload(t *testing.T) {
	fleet, agent, crd := pushFamilyArchives(t)
	pusher := &fakePusher{familyErr: errors.New("check whether reg.local/charts/fleet:111.0.0+up0.16.3 exists: i/o timeout")}

	stdout, _, _, err := runPushFamily(t, pusher, "--family", "fleet", fleet, agent, crd, "--registry", "reg.local/charts")

	require.ErrorContains(t, err, "nothing was pushed: check whether")
	require.Empty(t, stdout)
}

func TestPushFamilyRefusesBeforeUsingAnything(t *testing.T) {
	fleet, agent, crd := pushFamilyArchives(t)
	otherVersion := func() string {
		dir := writeChartDir(t, familyChartYAML("fleet-agent", "111.0.1+up0.16.3"), "")
		writeFiles(t, dir, map[string]string{"templates/deployment.yaml": "kind: Deployment\n"})
		archive, err := chart.Package(context.Background(), dir, filepath.Join(t.TempDir(), "other"))
		require.NoError(t, err)
		return archive
	}()
	longhornDir := writeChartDir(t, familyChartYAML("longhorn", packageFamilyVersion), "")
	writeFiles(t, longhornDir, map[string]string{"templates/deployment.yaml": "kind: Deployment\n"})
	longhorn, err := chart.Package(context.Background(), longhornDir, t.TempDir())
	require.NoError(t, err)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "a chart is missing",
			args:    []string{"--family", "fleet", fleet, crd, "--registry", "reg.local/charts"},
			wantErr: `the family "fleet" is missing fleet-agent`,
		},
		{
			name:    "a chart is given twice",
			args:    []string{"--family", "fleet", fleet, fleet, agent, crd, "--registry", "reg.local/charts"},
			wantErr: `chart "fleet" is given twice`,
		},
		{
			name:    "a chart of another family",
			args:    []string{"--family", "fleet", fleet, agent, crd, longhorn, "--registry", "reg.local/charts"},
			wantErr: `chart "longhorn" is not in the family "fleet"`,
		},
		{
			name:    "the versions differ",
			args:    []string{"--family", "fleet", fleet, otherVersion, crd, "--registry", "reg.local/charts"},
			wantErr: `the charts of the family "fleet" do not have the same version`,
		},
		{
			name:    "a family that is not listed",
			args:    []string{"--family", "nope", fleet, "--registry", "reg.local/charts"},
			wantErr: `family "nope" is not listed in the rules file`,
		},
		{
			name:    "no archives",
			args:    []string{"--family", "fleet", "--registry", "reg.local/charts"},
			wantErr: "requires at least 1 arg(s)",
		},
		{
			name:    "an archive that is not there",
			args:    []string{"--family", "fleet", "/nope/fleet.tgz", agent, crd, "--registry", "reg.local/charts"},
			wantErr: "read archive /nope/fleet.tgz",
		},
		{
			name:    "a registry the rules do not allow for these charts",
			args:    []string{"--family", "fleet", fleet, agent, crd, "--registry", "stgregistry.suse.com/rancher"},
			wantErr: "pushing to this registry is not allowed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pusher := &fakePusher{}

			stdout, _, built, err := runPushFamily(t, pusher, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.False(t, built, "no secret is read and no connection is made")
			require.Nil(t, pusher.familyArchives)
		})
	}
}

func TestPushWithoutAFamilyStillTakesOneArchive(t *testing.T) {
	fleet, agent, _ := pushFamilyArchives(t)

	_, _, _, err := runPushFamily(t, &fakePusher{}, fleet, agent, "--registry", "reg.local/charts")

	require.ErrorContains(t, err, "accepts 1 arg(s), received 2")
}
