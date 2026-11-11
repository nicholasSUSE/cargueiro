package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/check"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/ship"
)

// familyRules is checkRules with the fleet family of three charts, written with
// the main chart in the middle to show that the order comes from the rules.
func familyRules() string {
	return strings.NewReplacer(
		"kinds:\n  main:\n    required: [certified]\n",
		"kinds:\n  main:\n    required: [certified]\n  crd:\n    required: [certified]\n  extra:\n    required: [certified]\n",
		"      - { name: fleet, kind: main, edition: rancher-charts }\n",
		"      - { name: fleet-crd, kind: crd, edition: rancher-charts }\n      - { name: fleet, kind: main, edition: rancher-charts }\n      - { name: fleet-agent, kind: extra, edition: rancher-charts }\n",
	).Replace(checkRules)
}

const familyVersion = "110.0.3+up0.16.3"

// familyPreparer answers for each chart of a family and records what it was asked.
type familyPreparer struct {
	shipments map[string]ship.Shipment
	errs      map[string]error

	refs []oci.ChartRef
	opts []ship.Options
}

func (f *familyPreparer) prepare(_ context.Context, _ *rules.Config, ref oci.ChartRef, _ string, opts ship.Options) (ship.Shipment, error) {
	f.refs = append(f.refs, ref)
	f.opts = append(f.opts, opts)
	name := path.Base(ref.Repository)
	if err := f.errs[name]; err != nil {
		return ship.Shipment{}, err
	}
	return f.shipments[name], nil
}

// asked lists the charts that were asked for, in order.
func (f *familyPreparer) asked() []string {
	var names []string
	for _, ref := range f.refs {
		names = append(names, path.Base(ref.Repository))
	}
	return names
}

func fleetFamilyShipments() map[string]ship.Shipment {
	shipments := map[string]ship.Shipment{}
	for _, name := range []string{"fleet", "fleet-agent", "fleet-crd"} {
		shipments[name] = ship.Shipment{Chart: name, Version: familyVersion, Entry: []byte("name: " + name + "\n")}
	}
	fleet := shipments["fleet"]
	fleet.Logo = &ship.Logo{Data: []byte("<svg/>"), Extension: ".svg"}
	shipments["fleet"] = fleet
	return shipments
}

func runShipFamily(t *testing.T, fake *familyPreparer, verifier *fakeVerifier, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	return runShipFamilyWith(t, fake, verifier, nil, args...)
}

// runShipFamilyWith is runShipFamily with the given environment.
func runShipFamilyWith(t *testing.T, fake *familyPreparer, verifier *fakeVerifier, env map[string]string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(k string) string { return env[k] },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(familyRules()),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{prepare: fake.prepare, verifyCheckout: verifier.verify})
	require.NoError(t, err)
	root.SetArgs(append([]string{"ship"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func familyArgs(repo string, extra ...string) []string {
	return append([]string{"--family", "fleet", "--version", familyVersion, "--registry", "127.0.0.1:5052/e2e", "--repo", repo}, extra...)
}

func TestShipFamilyWritesEveryChart(t *testing.T) {
	fake := &familyPreparer{shipments: fleetFamilyShipments()}
	verifier := &fakeVerifier{}
	repo := t.TempDir()

	stdout, stderr, err := runShipFamily(t, fake, verifier, familyArgs(repo, "--plain-http")...)

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, strings.Join([]string{
		"entries/fleet/" + familyVersion + ".yaml",
		"assets/logos/fleet.svg",
		"entries/fleet-crd/" + familyVersion + ".yaml",
		"entries/fleet-agent/" + familyVersion + ".yaml",
	}, "\n")+"\n", stdout, "the main chart first, then the order of the rules")
	for _, name := range []string{"fleet", "fleet-agent", "fleet-crd"} {
		require.FileExists(t, filepath.Join(repo, "entries", name, familyVersion+".yaml"))
	}
	require.FileExists(t, filepath.Join(repo, "assets", "logos", "fleet.svg"))
	require.Equal(t, []string{"fleet", "fleet-crd", "fleet-agent"}, fake.asked())
	require.Equal(t, oci.ChartRef{Host: "127.0.0.1:5052", Repository: "e2e/fleet", Tag: "110.0.3_up0.16.3"}, fake.refs[0])
	for _, opts := range fake.opts {
		require.True(t, opts.PlainHTTP)
	}
}

func TestShipFamilyChecksTheCheckoutOnce(t *testing.T) {
	fake := &familyPreparer{shipments: fleetFamilyShipments()}
	verifier := &countingVerifier{}
	repo := t.TempDir()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(familyRules()),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{prepare: fake.prepare, verifyCheckout: verifier.verify})
	require.NoError(t, err)
	root.SetArgs(append([]string{"ship"}, familyArgs(repo)...))

	require.NoError(t, root.Execute())

	require.Equal(t, 1, verifier.calls, "three charts are written after one check")
	require.Equal(t, repo, verifier.dir)
	require.Equal(t, "staging", verifier.branch)
}

// countingVerifier counts the checks of the checkout.
type countingVerifier struct {
	calls  int
	dir    string
	branch string
}

func (c *countingVerifier) verify(_ context.Context, dir, branch string) error {
	c.calls++
	c.dir, c.branch = dir, branch
	return nil
}

func TestShipFamilyStopsWhenTheCheckoutIsNotRight(t *testing.T) {
	fake := &familyPreparer{shipments: fleetFamilyShipments()}
	verifier := &fakeVerifier{err: errors.New("the checkout is at aaa and staging is at bbb upstream: update it")}
	repo := t.TempDir()

	stdout, _, err := runShipFamily(t, fake, verifier, familyArgs(repo)...)

	require.EqualError(t, err, "the checkout is at aaa and staging is at bbb upstream: update it")
	require.Empty(t, stdout)
	require.Empty(t, fake.refs, "nothing is pulled")
	requireEmptyDir(t, repo)
}

func TestShipFamilyDryRunWritesNothing(t *testing.T) {
	fake := &familyPreparer{shipments: fleetFamilyShipments()}
	repo := t.TempDir()

	stdout, _, err := runShipFamily(t, fake, &fakeVerifier{}, familyArgs(repo, "--dry-run")...)

	require.NoError(t, err)
	require.Contains(t, stdout, "entries/fleet-agent/"+familyVersion+".yaml")
	requireEmptyDir(t, repo)
}

func TestShipFamilyWritesNothingWhenAChartCannotBePrepared(t *testing.T) {
	fake := &familyPreparer{
		shipments: fleetFamilyShipments(),
		errs:      map[string]error{"fleet-crd": errors.New("the registry has no fleet-crd:" + familyVersion)},
	}
	repo := t.TempDir()

	stdout, _, err := runShipFamily(t, fake, &fakeVerifier{}, familyArgs(repo)...)

	require.ErrorContains(t, err, "fleet-crd: the registry has no fleet-crd")
	require.Empty(t, stdout)
	require.Equal(t, []string{"fleet", "fleet-crd", "fleet-agent"}, fake.asked(), "every chart is tried")
	requireEmptyDir(t, repo)
}

func TestShipFamilyReportsEveryChartThatIsRefused(t *testing.T) {
	invalid := func(name string) error {
		return &ship.InvalidChartError{Result: &check.Result{Chart: name, Metadata: check.Metadata{Version: familyVersion}}}
	}
	fake := &familyPreparer{
		shipments: fleetFamilyShipments(),
		errs:      map[string]error{"fleet": invalid("fleet"), "fleet-agent": invalid("fleet-agent")},
	}
	repo := t.TempDir()

	_, stderr, err := runShipFamily(t, fake, &fakeVerifier{}, familyArgs(repo)...)

	require.ErrorContains(t, err, "fleet "+familyVersion+" does not follow the chart rules")
	require.ErrorContains(t, err, "fleet-agent "+familyVersion+" does not follow the chart rules")
	require.NotEmpty(t, stderr, "the reports are printed")
	requireEmptyDir(t, repo)
}

func TestShipFamilyRefusesAChartOfAnotherVersion(t *testing.T) {
	shipments := fleetFamilyShipments()
	other := shipments["fleet-crd"]
	other.Version = "110.0.4+up0.16.4"
	shipments["fleet-crd"] = other
	repo := t.TempDir()

	_, _, err := runShipFamily(t, &familyPreparer{shipments: shipments}, &fakeVerifier{}, familyArgs(repo)...)

	require.ErrorContains(t, err, "fleet-crd: the chart is version 110.0.4+up0.16.4, and the family is shipped at "+familyVersion)
	requireEmptyDir(t, repo)
}

func TestShipFamilyWritesNothingWhenOneEntryIsThere(t *testing.T) {
	fake := &familyPreparer{shipments: fleetFamilyShipments()}
	repo := t.TempDir()
	there := filepath.Join(repo, "entries", "fleet-agent", familyVersion+".yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(there), 0o750))
	require.NoError(t, os.WriteFile(there, []byte("old\n"), 0o600))

	stdout, _, err := runShipFamily(t, fake, &fakeVerifier{}, familyArgs(repo)...)

	require.ErrorContains(t, err, "entries/fleet-agent/"+familyVersion+".yaml already exists")
	require.Empty(t, stdout)
	require.NoFileExists(t, filepath.Join(repo, "entries", "fleet", familyVersion+".yaml"), "the charts before it are not written either")
	require.NoFileExists(t, filepath.Join(repo, "assets", "logos", "fleet.svg"))
	got, readErr := os.ReadFile(there)
	require.NoError(t, readErr)
	require.Equal(t, "old\n", string(got))
}

func TestShipFamilyRefuses(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "a reference with the family",
			args:    append([]string{"127.0.0.1:5052/e2e/fleet:" + familyVersion}, familyArgs("/tmp")...),
			wantErr: "give no reference with --family",
		},
		{
			name:    "no version",
			args:    []string{"--family", "fleet", "--registry", "127.0.0.1:5052/e2e", "--repo", "/tmp"},
			wantErr: "--family needs --version",
		},
		{
			name:    "no registry",
			args:    []string{"--family", "fleet", "--version", familyVersion, "--repo", "/tmp"},
			wantErr: "--family needs --registry",
		},
		{
			name:    "a version without a family",
			args:    []string{"127.0.0.1:5052/e2e/fleet:" + familyVersion, "--version", familyVersion, "--repo", "/tmp"},
			wantErr: "--version and --registry are only for --family",
		},
		{
			name:    "a family that is not listed",
			args:    []string{"--family", "nope", "--version", familyVersion, "--registry", "127.0.0.1:5052/e2e", "--repo", "/tmp"},
			wantErr: `family "nope" is not listed in the rules file`,
		},
		{
			name:    "a repository that is not an absolute path",
			args:    familyArgs("charts"),
			wantErr: "--repo must be an absolute path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &familyPreparer{shipments: fleetFamilyShipments()}
			verifier := &fakeVerifier{}

			stdout, _, err := runShipFamily(t, fake, verifier, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.False(t, verifier.called, "the arguments are checked before the checkout")
			require.Empty(t, fake.refs)
		})
	}
}

func requireEmptyDir(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestShipFamilyReadsEveryChartWithTheSameCredentials(t *testing.T) {
	const host = "127.0.0.1:5052"

	tests := []struct {
		name         string
		env          map[string]string
		args         []string
		wantUsername string
		wantPassword string
		wantErr      string
	}{
		{
			name: "none stays anonymous",
		},
		{
			name:         "username flag and password from the environment",
			env:          map[string]string{envPassword: "from-env"},
			args:         []string{"--username", "reader"},
			wantUsername: "reader",
			wantPassword: "from-env",
		},
		{
			name:         "both from the environment",
			env:          map[string]string{envUsername: "reader", envPassword: "from-env"},
			wantUsername: "reader",
			wantPassword: "from-env",
		},
		{
			name:    "a username without a password",
			args:    []string{"--username", "reader"},
			wantErr: "a password is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &familyPreparer{shipments: fleetFamilyShipments()}

			_, _, err := runShipFamilyWith(t, fake, &fakeVerifier{}, tt.env, familyArgs(t.TempDir(), tt.args...)...)

			require.NoError(t, err)
			require.Len(t, fake.opts, 3, "every chart is read")
			for _, opts := range fake.opts {
				require.NotNil(t, opts.Credentials)
				username, password, credErr := opts.Credentials(host)
				if tt.wantErr != "" {
					require.ErrorContains(t, credErr, tt.wantErr)
					continue
				}
				require.NoError(t, credErr)
				require.Equal(t, tt.wantUsername, username)
				require.Equal(t, tt.wantPassword, password)
			}
		})
	}
}
