package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/check"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
	"github.com/nicholasSUSE/cargueiro/internal/ship"
)

// fakePreparer answers with the shipment it is given and records what it was asked.
type fakePreparer struct {
	shipment ship.Shipment
	err      error

	called bool
	ref    oci.ChartRef
	dir    string
	opts   ship.Options
}

func (f *fakePreparer) prepare(_ context.Context, _ *rules.Config, ref oci.ChartRef, dir string, opts ship.Options) (ship.Shipment, error) {
	f.called = true
	f.ref, f.dir, f.opts = ref, dir, opts
	return f.shipment, f.err
}

func fleetShipment() ship.Shipment {
	return ship.Shipment{
		Chart:   "fleet",
		Version: "110.0.3+up0.16.3",
		Entry:   []byte("name: fleet\n"),
		Logo:    &ship.Logo{Data: []byte("<svg/>"), Extension: ".svg"},
	}
}

// runShip executes ship with the rules in checkRules.
func runShip(t *testing.T, fake *fakePreparer, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{prepare: fake.prepare})
	require.NoError(t, err)
	root.SetArgs(append([]string{"ship"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

const shipReference = "127.0.0.1:5052/e2e/fleet:110.0.3+up0.16.3"

func TestShipWritesTheEntryAndTheLogo(t *testing.T) {
	fake := &fakePreparer{shipment: fleetShipment()}
	repo := t.TempDir()

	stdout, stderr, err := runShip(t, fake, shipReference, "--repo", repo, "--plain-http")

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "entries/fleet/110.0.3+up0.16.3.yaml\nassets/logos/fleet.svg\n", stdout)
	got, readErr := os.ReadFile(filepath.Join(repo, "entries", "fleet", "110.0.3+up0.16.3.yaml"))
	require.NoError(t, readErr)
	require.Equal(t, "name: fleet\n", string(got))
	require.FileExists(t, filepath.Join(repo, "assets", "logos", "fleet.svg"))
	require.True(t, fake.called)
	require.Equal(t, oci.ChartRef{Host: "127.0.0.1:5052", Repository: "e2e/fleet", Tag: "110.0.3_up0.16.3"}, fake.ref)
	require.Equal(t, repo, fake.dir)
	require.True(t, fake.opts.PlainHTTP)
}

func TestShipWithoutPlainHTTP(t *testing.T) {
	fake := &fakePreparer{shipment: fleetShipment()}

	_, _, err := runShip(t, fake, shipReference, "--repo", t.TempDir())

	require.NoError(t, err)
	require.False(t, fake.opts.PlainHTTP)
}

func TestShipDryRunSaysWhatItWouldWriteAndWritesNothing(t *testing.T) {
	fake := &fakePreparer{shipment: fleetShipment()}
	repo := t.TempDir()

	stdout, _, err := runShip(t, fake, shipReference, "--repo", repo, "--dry-run")

	require.NoError(t, err)
	require.Equal(t, "entries/fleet/110.0.3+up0.16.3.yaml\nassets/logos/fleet.svg\n", stdout)
	entries, readErr := os.ReadDir(repo)
	require.NoError(t, readErr)
	require.Empty(t, entries)
}

func TestShipRefusesAnEntryThatIsThere(t *testing.T) {
	fake := &fakePreparer{shipment: fleetShipment()}
	repo := t.TempDir()
	entry := filepath.Join(repo, "entries", "fleet", "110.0.3+up0.16.3.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(entry), 0o750))
	require.NoError(t, os.WriteFile(entry, []byte("released"), 0o600))

	stdout, _, err := runShip(t, fake, shipReference, "--repo", repo)

	require.ErrorContains(t, err, "an entry is never replaced")
	require.Empty(t, stdout)
	got, readErr := os.ReadFile(entry)
	require.NoError(t, readErr)
	require.Equal(t, "released", string(got))
}

func TestShipPrintsWhyAChartIsRefused(t *testing.T) {
	fake := &fakePreparer{err: &ship.InvalidChartError{Result: &check.Result{
		Chart:       "fleet",
		Metadata:    check.Metadata{Name: "fleet", Version: "110.0.3+up0.16.3"},
		Annotations: check.Section{Errors: []string{"catalog.cattle.io/certified is missing"}},
	}}}
	repo := t.TempDir()

	stdout, stderr, err := runShip(t, fake, shipReference, "--repo", repo)

	require.ErrorContains(t, err, "fleet 110.0.3+up0.16.3 does not follow the chart rules")
	require.Empty(t, stdout)
	require.Contains(t, stderr, "catalog.cattle.io/certified is missing")
	entries, readErr := os.ReadDir(repo)
	require.NoError(t, readErr)
	require.Empty(t, entries, "nothing is written")
}

func TestShipPassesOnWhatPrepareSays(t *testing.T) {
	fake := &fakePreparer{err: errors.New("the registry is down")}

	stdout, _, err := runShip(t, fake, shipReference, "--repo", t.TempDir())

	require.EqualError(t, err, "the registry is down")
	require.Empty(t, stdout)
}

func TestShipRefuses(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no reference",
			args:    []string{"--repo", "/tmp"},
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name:    "no repository",
			args:    []string{shipReference},
			wantErr: `required flag(s) "repo" not set`,
		},
		{
			name:    "a repository that is not an absolute path",
			args:    []string{shipReference, "--repo", "charts"},
			wantErr: "--repo must be an absolute path",
		},
		{
			name:    "a reference with no tag",
			args:    []string{"127.0.0.1:5052/e2e/fleet", "--repo", "/tmp"},
			wantErr: "has no tag",
		},
		{
			name:    "a reference by digest",
			args:    []string{"127.0.0.1:5052/e2e/fleet@sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "--repo", "/tmp"},
			wantErr: "references by digest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakePreparer{shipment: fleetShipment()}

			stdout, _, err := runShip(t, fake, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.False(t, fake.called)
		})
	}
}
