package cmd

import (
	"bytes"
	"context"
	"errors"
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

func TestPushCmdShowsEachHintOnce(t *testing.T) {
	untrusted := &oci.RegistryError{Kind: oci.KindUntrustedCertificate, Err: errors.New("x509: unknown authority")}
	failing := map[string]error{"a.tgz": untrusted, "b.tgz": untrusted, "c.tgz": untrusted}

	res := runPush(t, nil, "", failing, "a.tgz", "b.tgz", "c.tgz", "--registry", "reg.local")

	require.ErrorContains(t, res.err, "3 of 3 archives failed")
	require.Equal(t, 1, strings.Count(res.err.Error(), "hint: "), "one hint for three identical failures")
	require.ErrorContains(t, res.err, "--ca-file")
	require.ErrorContains(t, res.err, "x509: unknown authority", "the original error is still shown")
	require.ErrorIs(t, res.err, untrusted)
}

func TestPushCmdHintMentionsTheUser(t *testing.T) {
	rejected := &oci.RegistryError{Kind: oci.KindAuthRejected, Err: errors.New("401")}

	res := runPush(t, nil, "", map[string]error{"a.tgz": rejected}, "a.tgz", "--registry", "reg.local", "--username", "admin")

	require.ErrorContains(t, res.err, `credentials of user "admin"`)
}

func TestPushCmdWithoutHint(t *testing.T) {
	res := runPush(t, nil, "", map[string]error{"a.tgz": errors.New("boom")}, "a.tgz", "--registry", "reg.local")

	require.ErrorContains(t, res.err, "boom")
	require.NotContains(t, res.err.Error(), "hint:")
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

func TestPushCmdUsesSavedCredentials(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	_, err := store.Put(ctx, "reg.local:5000", auth.Credential{Username: "saved-user", Password: "saved-pass"})
	require.NoError(t, err)

	tests := []struct {
		name string
		env  map[string]string
		args []string
		want oci.ClientOptions
	}{
		{
			name: "no credentials given: the saved ones are used",
			args: []string{"--registry", "reg.local:5000/charts"},
			want: oci.ClientOptions{Username: "saved-user", Password: "saved-pass"},
		},
		{
			name: "host matching ignores case",
			args: []string{"--registry", "REG.local:5000/charts"},
			want: oci.ClientOptions{Username: "saved-user", Password: "saved-pass"},
		},
		{
			name: "username flag beats saved credentials",
			env:  map[string]string{"CARGUEIRO_PASSWORD": "env-pass"},
			args: []string{"--registry", "reg.local:5000/charts", "--username", "flag-user"},
			want: oci.ClientOptions{Username: "flag-user", Password: "env-pass"},
		},
		{
			name: "environment beats saved credentials",
			env:  map[string]string{"CARGUEIRO_USERNAME": "env-user", "CARGUEIRO_PASSWORD": "env-pass"},
			args: []string{"--registry", "reg.local:5000/charts"},
			want: oci.ClientOptions{Username: "env-user", Password: "env-pass"},
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
			name: "same host without the port gets nothing",
			args: []string{"--registry", "reg.local/charts"},
			want: oci.ClientOptions{},
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

func TestPushCmdHintsToLoginWhenSavedCredentialsAreRejected(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	_, err := store.Put(ctx, "reg.local", auth.Credential{Username: "u", Password: "p"})
	require.NoError(t, err)
	rejected := &oci.RegistryError{Kind: oci.KindAuthRejected, Err: errors.New("401")}

	res := runPushWithStore(t, store, nil, "", map[string]error{"a.tgz": rejected}, "a.tgz", "--registry", "reg.local")

	require.ErrorContains(t, res.err, "hint: the registry rejected the saved credentials for reg.local")
	require.ErrorContains(t, res.err, "registry login reg.local")
}

func TestPushCmdHintsToLoginWhenNothingIsSaved(t *testing.T) {
	required := &oci.RegistryError{Kind: oci.KindAuthRequired, Err: errors.New("credential not found")}

	res := runPush(t, nil, "", map[string]error{"a.tgz": required}, "a.tgz", "--registry", "reg.local:5000/charts")

	require.ErrorContains(t, res.err, "registry login reg.local:5000")
}

func TestPushCmdFailsOnACorruptCredentialsFile(t *testing.T) {
	broken := func() (*auth.Store, error) { return nil, errors.New("credentials file is corrupt") }
	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: broken,
		prompt:    &fakePrompter{},
	}
	factoryCalled := false
	factory := func(oci.Target, oci.ClientOptions) (archivePusher, error) {
		factoryCalled = true
		return &fakePusher{}, nil
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{newPusher: factory})
	require.NoError(t, err)
	root.SetArgs([]string{"push", "a.tgz", "--registry", "reg.local"})

	err = root.Execute()

	require.ErrorContains(t, err, "credentials file is corrupt")
	require.False(t, factoryCalled, "a broken credentials file must not silently fall back to anonymous")
}

func TestPushCmdDoesNotReadTheStoreWhenCredentialsAreGiven(t *testing.T) {
	broken := func() (*auth.Store, error) { return nil, errors.New("must not be opened") }
	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr: &errOut,
		getenv: func(k string) string {
			return map[string]string{"CARGUEIRO_USERNAME": "u", "CARGUEIRO_PASSWORD": "p"}[k]
		},
		openStore: broken,
		prompt:    &fakePrompter{},
	}
	fake := &fakePusher{}
	factory := func(oci.Target, oci.ClientOptions) (archivePusher, error) { return fake, nil }
	root, err := newRootCmd(&out, &errOut, opts, dependencies{newPusher: factory})
	require.NoError(t, err)
	root.SetArgs([]string{"push", "a.tgz", "--registry", "reg.local"})

	require.NoError(t, root.Execute())
}
