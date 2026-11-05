package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
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

	return runPushWithStore(t, newTestStore(t), env, stdin, failing, args...)
}

// runPushWithStore is runPush with the saved credentials in store.
func runPushWithStore(t *testing.T, store *auth.Store, env map[string]string, stdin string, failing map[string]error, args ...string) pushResult {
	t.Helper()

	res := pushResult{pusher: &fakePusher{failing: failing}}
	factory := func(target oci.Target, opts oci.ClientOptions) (archivePusher, error) {
		res.env = pushEnv{target: target, opts: opts}
		return res.pusher, nil
	}

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(k string) string { return env[k] },
		openStore: openerFor(store),
		prompt:    &fakePrompter{},
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{newPusher: factory})
	require.NoError(t, err)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"push"}, args...))

	res.err = root.Execute()
	res.stdout, res.stderr = out.String(), errOut.String()
	return res
}

func TestPushCmdPrintsReferences(t *testing.T) {
	res := runPush(t, nil, "", nil, "a.tgz", "--registry", "reg.local/charts")

	require.NoError(t, res.err)
	require.Equal(t, "oci://reg.local/charts/a:1.0.0\n", res.stdout)
	require.Equal(t, []string{"a.tgz"}, res.pusher.pushed)
	require.Equal(t, oci.Target{Host: "reg.local", Path: "charts"}, res.env.target)
}

func TestPushCmdReturnsThePushError(t *testing.T) {
	errBoom := errors.New("boom")

	res := runPush(t, nil, "", map[string]error{"a.tgz": errBoom}, "a.tgz", "--registry", "reg.local")

	require.ErrorIs(t, res.err, errBoom)
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
			env:  map[string]string{"CARGUEIRO_USERNAME": "env-user", "CARGUEIRO_PASSWORD": "env-secret"},
			args: []string{"--registry", "r", "-u", "flag-user"},
			want: oci.ClientOptions{Username: "flag-user", Password: "env-secret"},
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
		env     map[string]string
		stdin   string
		wantErr string
	}{
		{name: "no registry", args: []string{"a.tgz"}, wantErr: `required flag(s) "registry" not set`},
		{name: "no archive", args: []string{"--registry", "r"}, wantErr: "accepts 1 arg(s), received 0"},
		{name: "two archives", args: []string{"a.tgz", "b.tgz", "--registry", "r"}, wantErr: "accepts 1 arg(s), received 2"},
		{name: "invalid registry", args: []string{"a.tgz", "--registry", "host/a//b"}, wantErr: "invalid registry"},
		{name: "username without password", args: []string{"a.tgz", "--registry", "r", "-u", "admin"}, wantErr: "a password is required"},
		{name: "password without username", args: []string{"a.tgz", "--registry", "r"}, env: map[string]string{"CARGUEIRO_PASSWORD": "p"}, wantErr: "a username is required"},
		{name: "password too long", args: []string{"a.tgz", "--registry", "r", "--password-stdin"}, stdin: strings.Repeat("x", 5000), wantErr: "longer than"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runPush(t, tt.env, tt.stdin, nil, tt.args...)

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
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
	}
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

func TestPushCmdRefusesProtectedRegistries(t *testing.T) {
	for _, registry := range []string{
		"registry.suse.com/rancher/catalog",
		"stgregistry.suse.com/rancher/catalog",
		"https://Registry.SUSE.com:443/rancher/catalog/",
	} {
		t.Run(registry, func(t *testing.T) {
			res := runPush(t, map[string]string{"CARGUEIRO_PASSWORD": "secret"}, "", nil,
				"a.tgz", "--registry", registry, "--username", "admin")

			require.ErrorIs(t, res.err, oci.ErrProtectedRegistry)
			require.Equal(t, pushEnv{}, res.env, "no pusher may even be built")
			require.Empty(t, res.pusher.pushed)
			require.Empty(t, res.stdout)
		})
	}
}

func TestPushCmdAllowsTheSamePathOnOtherRegistries(t *testing.T) {
	res := runPush(t, nil, "", nil, "a.tgz", "--registry", "localhost:5051/rancher/catalog")

	require.NoError(t, res.err)
	require.Equal(t, oci.Target{Host: "localhost:5051", Path: "rancher/catalog"}, res.env.target)
}

func saveLogin(t *testing.T, store *auth.Store, login auth.Login) {
	t.Helper()

	require.NoError(t, store.Save(login))
}

func TestPushCmdUsesTheSavedLogin(t *testing.T) {
	store := newTestStore(t)
	saveLogin(t, store, auth.Login{Host: "reg.local:5000", Username: "saved-user", Password: "saved-pass", PlainHTTP: true})

	tests := []struct {
		name string
		env  map[string]string
		args []string
		want oci.ClientOptions
	}{
		{
			name: "no credentials given: the saved ones and the saved transport are used",
			args: []string{"--registry", "reg.local:5000/charts"},
			want: oci.ClientOptions{Username: "saved-user", Password: "saved-pass", PlainHTTP: true},
		},
		{
			name: "host matching ignores case",
			args: []string{"--registry", "REG.local:5000/charts"},
			want: oci.ClientOptions{Username: "saved-user", Password: "saved-pass", PlainHTTP: true},
		},
		{
			name: "username flag beats the saved credentials but not the saved transport",
			env:  map[string]string{"CARGUEIRO_PASSWORD": "env-pass"},
			args: []string{"--registry", "reg.local:5000/charts", "--username", "flag-user"},
			want: oci.ClientOptions{Username: "flag-user", Password: "env-pass", PlainHTTP: true},
		},
		{
			name: "environment beats the saved credentials",
			env:  map[string]string{"CARGUEIRO_USERNAME": "env-user", "CARGUEIRO_PASSWORD": "env-pass"},
			args: []string{"--registry", "reg.local:5000/charts"},
			want: oci.ClientOptions{Username: "env-user", Password: "env-pass", PlainHTTP: true},
		},
		{
			name: "another host gets nothing",
			args: []string{"--registry", "other.local:5000/charts"},
			want: oci.ClientOptions{},
		},
		{
			name: "another port gets nothing",
			args: []string{"--registry", "reg.local:5001/charts"},
			want: oci.ClientOptions{},
		},
		{
			name: "the same host without the port gets nothing",
			args: []string{"--registry", "reg.local/charts"},
			want: oci.ClientOptions{},
		},
		{
			name: "another host with explicit credentials gets only those",
			env:  map[string]string{"CARGUEIRO_USERNAME": "u", "CARGUEIRO_PASSWORD": "p"},
			args: []string{"--registry", "other.local:5000/charts"},
			want: oci.ClientOptions{Username: "u", Password: "p"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runPushWithStore(t, store, tt.env, "", nil, append([]string{"a.tgz"}, tt.args...)...)

			require.NoError(t, res.err)
			require.Equal(t, tt.want, res.env.opts)
		})
	}
}

func TestPushCmdWarnsWhenOthersCanReadTheLoginFile(t *testing.T) {
	store := newTestStore(t)
	saveLogin(t, store, auth.Login{Host: "reg.local:5000", Username: "saved-user", Password: "saved-pass", PlainHTTP: true})
	require.NoError(t, os.Chmod(store.Path(), 0o644))

	res := runPushWithStore(t, store, nil, "", nil, "a.tgz", "--registry", "reg.local:5000/charts")

	require.NoError(t, res.err, "the push still goes ahead")
	require.Equal(t, oci.ClientOptions{Username: "saved-user", Password: "saved-pass", PlainHTTP: true}, res.env.opts)
	require.Contains(t, res.stderr, "warning: "+auth.ErrInsecurePermissions.Error())
	require.NotContains(t, res.stderr, "saved-pass")
}

func TestPushCmdUsesTheSavedCA(t *testing.T) {
	store := newTestStore(t)
	saveLogin(t, store, auth.Login{Host: "secure.test:5051", Username: "u", Password: "p", CAPEM: testCAPEM})

	res := runPushWithStore(t, store, nil, "", nil, "a.tgz", "--registry", "secure.test:5051/charts")

	require.NoError(t, res.err)
	require.Equal(t, oci.ClientOptions{Username: "u", Password: "p", CAPEM: []byte(testCAPEM)}, res.env.opts)
}

func TestPushCmdWithNoLoginIsAnonymousOverHTTPS(t *testing.T) {
	res := runPush(t, nil, "", nil, "a.tgz", "--registry", "reg.local")

	require.NoError(t, res.err)
	require.Equal(t, oci.ClientOptions{}, res.env.opts)
}

func TestPushCmdFailsOnACorruptLoginFile(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"no credentials given":      nil,
		"with explicit credentials": {"CARGUEIRO_USERNAME": "u", "CARGUEIRO_PASSWORD": "p"},
	} {
		t.Run(name, func(t *testing.T) {
			store := newTestStore(t)
			require.NoError(t, os.MkdirAll(filepath.Dir(store.Path()), 0o700))
			require.NoError(t, os.WriteFile(store.Path(), []byte("{ not json"), 0o600))

			res := runPushWithStore(t, store, env, "", nil, "a.tgz", "--registry", "reg.local")

			require.ErrorContains(t, res.err, "registry logout", "it says how to recover")
			require.Equal(t, pushEnv{}, res.env, "the saved transport cannot be known, so do nothing")
			require.Empty(t, res.pusher.pushed)
		})
	}
}
