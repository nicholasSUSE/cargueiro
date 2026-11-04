package cmd

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
)

type call struct{ src, dest string }

// fakePackager records its calls and fails for any source listed in failing.
type fakePackager struct {
	calls   []call
	failing map[string]error
}

func (f *fakePackager) pkg(_ context.Context, src, dest string) (string, error) {
	f.calls = append(f.calls, call{src: src, dest: dest})
	if err := f.failing[src]; err != nil {
		return "", err
	}
	return filepath.Join("/abs", dest, filepath.Base(src)+".tgz"), nil
}

// run executes a root command wired to the fake packager.
func run(t *testing.T, fake *fakePackager, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{pkg: fake.pkg, newPusher: nil})
	require.NoError(t, err)
	root.SetArgs(args)

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestPackageCmd(t *testing.T) {
	errBoom := errors.New("boom")

	tests := []struct {
		name       string
		args       []string
		failing    map[string]error
		wantStdout string
		wantCalls  []call
		wantErr    string
	}{
		{
			name:       "single chart, default output dir",
			args:       []string{"package", "charts/fleet"},
			wantStdout: "/abs/assets/fleet.tgz\n",
			wantCalls:  []call{{src: "charts/fleet", dest: "assets"}},
		},
		{
			name:       "custom output dir",
			args:       []string{"package", "charts/fleet", "-o", "out"},
			wantStdout: "/abs/out/fleet.tgz\n",
			wantCalls:  []call{{src: "charts/fleet", dest: "out"}},
		},
		{
			name:       "several charts keep argument order",
			args:       []string{"package", "charts/fleet-crd", "charts/fleet", "charts/fleet-agent"},
			wantStdout: "/abs/assets/fleet-crd.tgz\n/abs/assets/fleet.tgz\n/abs/assets/fleet-agent.tgz\n",
			wantCalls: []call{
				{src: "charts/fleet-crd", dest: "assets"},
				{src: "charts/fleet", dest: "assets"},
				{src: "charts/fleet-agent", dest: "assets"},
			},
		},
		{
			name:       "one failure does not stop the others",
			args:       []string{"package", "charts/a", "charts/bad", "charts/c"},
			failing:    map[string]error{"charts/bad": errBoom},
			wantStdout: "/abs/assets/a.tgz\n/abs/assets/c.tgz\n",
			wantCalls: []call{
				{src: "charts/a", dest: "assets"},
				{src: "charts/bad", dest: "assets"},
				{src: "charts/c", dest: "assets"},
			},
			wantErr: "boom",
		},
		{
			name:    "all fail, nothing on stdout",
			args:    []string{"package", "charts/x"},
			failing: map[string]error{"charts/x": errBoom},
			wantCalls: []call{
				{src: "charts/x", dest: "assets"},
			},
			wantErr: "boom",
		},
		{
			name:    "no chart given",
			args:    []string{"package"},
			wantErr: "requires at least 1 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakePackager{failing: tt.failing}

			stdout, _, err := run(t, fake, tt.args...)

			require.Equal(t, tt.wantStdout, stdout)
			require.Equal(t, tt.wantCalls, fake.calls)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPackageCmdLogsToStderr(t *testing.T) {
	fake := &fakePackager{}

	stdout, stderr, err := run(t, fake, "--debug", "package", "charts/fleet")

	require.NoError(t, err)
	require.Equal(t, "/abs/assets/fleet.tgz\n", stdout)
	require.Contains(t, stderr, "packaging chart")
}

func TestPackageCmdEndToEnd(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "out")

	var out, errOut bytes.Buffer
	root, err := NewRootCmd(&out, &errOut)
	require.NoError(t, err)
	root.SetArgs([]string{"package", "../internal/chart/testdata/valid", "-o", dest})

	require.NoError(t, root.Execute())
	require.Equal(t, filepath.Join(dest, "valid-0.1.0.tgz")+"\n", out.String())

	// The real packager rejects a directory that is not a chart.
	root, err = NewRootCmd(&out, &errOut)
	require.NoError(t, err)
	root.SetArgs([]string{"package", "../internal/chart/testdata/noyaml", "-o", dest})
	require.ErrorIs(t, root.Execute(), chart.ErrNoChartYaml)
}
