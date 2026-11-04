package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// fakePusher records pushes and fails for archives listed in failing.
type fakePusher struct {
	pushed  []string
	failing map[string]error
}

func (f *fakePusher) Push(_ context.Context, archive string) (string, error) {
	f.pushed = append(f.pushed, archive)
	if err := f.failing[archive]; err != nil {
		return "", err
	}
	return "reg.local/charts/" + strings.TrimSuffix(archive, ".tgz") + ":1.0.0", nil
}

// pushEnv is what a pusherFactory call received.
type pushEnv struct {
	target oci.Target
	opts   oci.ClientOptions
}

type pushResult struct {
	stdout string
	stderr string
	err    error
	env    pushEnv
	pusher *fakePusher
}

// runPush executes `push` with a fake pusher factory. env is the process
// environment seen by the command, stdin its standard input.
func runPush(t *testing.T, env map[string]string, stdin string, failing map[string]error, args ...string) pushResult {
	t.Helper()

	res := pushResult{pusher: &fakePusher{failing: failing}}
	factory := func(target oci.Target, opts oci.ClientOptions) (archivePusher, error) {
		res.env = pushEnv{target: target, opts: opts}
		return res.pusher, nil
	}

	var out, errOut bytes.Buffer
	opts := &rootOptions{stderr: &errOut, getenv: func(k string) string { return env[k] }}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{newPusher: factory})
	require.NoError(t, err)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"push"}, args...))

	res.err = root.Execute()
	res.stdout, res.stderr = out.String(), errOut.String()
	return res
}

func TestPushCmdPrintsReferences(t *testing.T) {
	res := runPush(t, nil, "", nil, "a.tgz", "b.tgz", "--registry", "reg.local/charts")

	require.NoError(t, res.err)
	require.Equal(t, "oci://reg.local/charts/a:1.0.0\noci://reg.local/charts/b:1.0.0\n", res.stdout)
	require.Equal(t, []string{"a.tgz", "b.tgz"}, res.pusher.pushed, "argument order")
	require.Equal(t, oci.Target{Host: "reg.local", Path: "charts"}, res.env.target)
}

func TestPushCmdAttemptsEveryArchive(t *testing.T) {
	errBoom := errors.New("boom")

	res := runPush(t, nil, "", map[string]error{"bad.tgz": errBoom}, "a.tgz", "bad.tgz", "c.tgz", "--registry", "reg.local")

	require.Error(t, res.err)
	require.ErrorIs(t, res.err, errBoom)
	require.ErrorContains(t, res.err, "1 of 3 archives failed")
	require.Equal(t, []string{"a.tgz", "bad.tgz", "c.tgz"}, res.pusher.pushed)
	require.Equal(t, "oci://reg.local/charts/a:1.0.0\noci://reg.local/charts/c:1.0.0\n", res.stdout, "successes still reported")
}

func TestPushCmdReportsEveryFailure(t *testing.T) {
	errA, errB := errors.New("first failure"), errors.New("second failure")

	res := runPush(t, nil, "", map[string]error{"a.tgz": errA, "b.tgz": errB}, "a.tgz", "b.tgz", "--registry", "reg.local")

	require.ErrorContains(t, res.err, "2 of 2 archives failed")
	require.ErrorContains(t, res.err, "first failure")
	require.ErrorContains(t, res.err, "second failure")
	require.Empty(t, res.stdout)
}

func TestPushCmdCredentials(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		stdin string
		args  []string
		want  oci.ClientOptions
	}{
		{
			name: "anonymous",
			args: []string{"--registry", "r"},
			want: oci.ClientOptions{},
		},
		{
			name: "username flag and password from environment",
			env:  map[string]string{"CARGUEIRO_PASSWORD": "env-secret"},
			args: []string{"--registry", "r", "--username", "admin"},
			want: oci.ClientOptions{Username: "admin", Password: "env-secret"},
		},
		{
			name: "username from environment",
			env:  map[string]string{"CARGUEIRO_USERNAME": "env-user", "CARGUEIRO_PASSWORD": "env-secret"},
			args: []string{"--registry", "r"},
			want: oci.ClientOptions{Username: "env-user", Password: "env-secret"},
		},
		{
			name: "flag beats environment username",
			env:  map[string]string{"CARGUEIRO_USERNAME": "env-user"},
			args: []string{"--registry", "r", "-u", "flag-user"},
			want: oci.ClientOptions{Username: "flag-user"},
		},
		{
			name:  "password from stdin, trailing newline removed",
			stdin: "stdin-secret\n",
			args:  []string{"--registry", "r", "--username", "admin", "--password-stdin"},
			want:  oci.ClientOptions{Username: "admin", Password: "stdin-secret"},
		},
		{
			name:  "stdin beats environment password",
			env:   map[string]string{"CARGUEIRO_PASSWORD": "env-secret"},
			stdin: "stdin-secret\r\n",
			args:  []string{"--registry", "r", "--username", "admin", "--password-stdin"},
			want:  oci.ClientOptions{Username: "admin", Password: "stdin-secret"},
		},
		{
			name: "transport flags",
			args: []string{"--registry", "r", "--plain-http"},
			want: oci.ClientOptions{PlainHTTP: true},
		},
		{
			name: "ca file",
			args: []string{"--registry", "r", "--ca-file", "ca.crt"},
			want: oci.ClientOptions{CAFile: "ca.crt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runPush(t, tt.env, tt.stdin, nil, append([]string{"a.tgz"}, tt.args...)...)

			require.NoError(t, res.err)
			require.Equal(t, tt.want, res.env.opts)
		})
	}
}

func TestPushCmdRejectsBadInput(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		stdin   string
		wantErr string
	}{
		{name: "no registry", args: []string{"a.tgz"}, wantErr: `required flag(s) "registry" not set`},
		{name: "no archive", args: []string{"--registry", "r"}, wantErr: "requires at least 1 arg"},
		{name: "invalid registry", args: []string{"a.tgz", "--registry", "host/a//b"}, wantErr: "invalid registry"},
		{name: "password too long", args: []string{"a.tgz", "--registry", "r", "--password-stdin"}, stdin: strings.Repeat("x", 5000), wantErr: "longer than"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runPush(t, nil, tt.stdin, nil, tt.args...)

			require.ErrorContains(t, res.err, tt.wantErr)
			require.Empty(t, res.pusher.pushed, "nothing may be pushed")
			require.Empty(t, res.stdout)
		})
	}
}

func TestPushCmdFactoryError(t *testing.T) {
	errBadOpts := errors.New("bad options")
	factory := func(oci.Target, oci.ClientOptions) (archivePusher, error) { return nil, errBadOpts }

	var out, errOut bytes.Buffer
	opts := &rootOptions{stderr: &errOut, getenv: func(string) string { return "" }}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{newPusher: factory})
	require.NoError(t, err)
	root.SetArgs([]string{"push", "a.tgz", "--registry", "r"})

	require.ErrorIs(t, root.Execute(), errBadOpts)
}

func TestPushCmdSecretsNeverInOutput(t *testing.T) {
	res := runPush(t, map[string]string{"CARGUEIRO_PASSWORD": "super-secret"}, "", nil,
		"--debug", "a.tgz", "--registry", "r", "--username", "admin")

	require.NoError(t, res.err)
	require.NotContains(t, res.stdout, "super-secret")
	require.NotContains(t, res.stderr, "super-secret")
}
