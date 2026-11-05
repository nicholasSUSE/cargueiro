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

const testCAPEM = "-----BEGIN CERTIFICATE-----\nMIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw\n-----END CERTIFICATE-----\n"

// verifyCall is what a fake verifyFunc was asked.
type verifyCall struct {
	host string
	opts oci.ClientOptions
}

type registryResult struct {
	stdout string
	stderr string
	err    error
	calls  []verifyCall
}

// runRegistry executes a `registry ...` command with fakes for the login
// store, the prompter and the login check.
func runRegistry(t *testing.T, store *auth.Store, p *fakePrompter, env map[string]string, stdin string,
	verified bool, verifyErr error, args ...string) registryResult {
	t.Helper()

	var res registryResult
	verify := func(_ context.Context, host string, opts oci.ClientOptions) (bool, error) {
		res.calls = append(res.calls, verifyCall{host: host, opts: opts})
		return verified, verifyErr
	}

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(k string) string { return env[k] },
		openStore: openerFor(store),
		prompt:    p,
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{verifyLogin: verify})
	require.NoError(t, err)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"registry"}, args...))

	res.err = root.Execute()
	res.stdout, res.stderr = out.String(), errOut.String()
	return res
}

func saved(t *testing.T, store *auth.Store) (auth.Login, bool) {
	t.Helper()

	login, found, err := store.Load()
	require.NoError(t, err)
	return login, found
}

func requireNoSecret(t *testing.T, res registryResult, secrets ...string) {
	t.Helper()

	for _, secret := range secrets {
		require.NotContains(t, res.stdout, secret)
		require.NotContains(t, res.stderr, secret)
		if res.err != nil {
			require.NotContains(t, res.err.Error(), secret)
		}
	}
}

func TestLoginSavesTheVerifiedLogin(t *testing.T) {
	store := newTestStore(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "s3cret-pass\n", true, nil,
		"login", "reg.example.com:5000", "--username", "admin", "--password-stdin", "--plain-http")

	require.NoError(t, res.err)
	require.Equal(t, "Login succeeded for reg.example.com:5000\n", res.stdout)
	require.Equal(t, []verifyCall{{
		host: "reg.example.com:5000",
		opts: oci.ClientOptions{Username: "admin", Password: "s3cret-pass", PlainHTTP: true},
	}}, res.calls, "the credentials are checked against the registry first")
	login, found := saved(t, store)
	require.True(t, found)
	require.Equal(t, auth.Login{Host: "reg.example.com:5000", Username: "admin", Password: "s3cret-pass", PlainHTTP: true}, login)
	require.Contains(t, res.stderr, "saved in plain text in "+store.Path())
	requireNoSecret(t, res, "s3cret-pass")
}

func TestLoginHostForms(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantHost string
	}{
		{name: "plain host", arg: "reg.example.com", wantHost: "reg.example.com"},
		{name: "scheme is ignored", arg: "https://reg.example.com", wantHost: "reg.example.com"},
		{name: "oci scheme and slash", arg: "oci://reg.example.com/", wantHost: "reg.example.com"},
		{name: "case is kept as typed", arg: "Reg.Example.COM:5000", wantHost: "Reg.Example.COM:5000"},
		{name: "a protected SUSE host can be logged in to", arg: "registry.suse.com", wantHost: "registry.suse.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)

			res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", true, nil,
				"login", tt.arg, "-u", "admin", "--password-stdin")

			require.NoError(t, res.err)
			require.Equal(t, tt.wantHost, res.calls[0].host)
			login, _ := saved(t, store)
			require.Equal(t, tt.wantHost, login.Host)
		})
	}
}

func TestLoginRefusesAnAddressWithAPath(t *testing.T) {
	store := newTestStore(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", true, nil,
		"login", "reg.example.com/rancher/catalog", "-u", "admin", "--password-stdin")

	require.ErrorIs(t, res.err, oci.ErrInvalidRegistry)
	require.ErrorContains(t, res.err, `use "reg.example.com"`)
	require.Empty(t, res.calls, "no network call")
	require.NoFileExists(t, store.Path())
}

func TestLoginWhenAlreadyLoggedInAsksToLogOutFirst(t *testing.T) {
	for _, host := range []string{"a.example.com", "b.example.com"} {
		t.Run(host, func(t *testing.T) {
			store := newTestStore(t)
			existing := auth.Login{Host: "a.example.com", Username: "alice", Password: "pa", PlainHTTP: true}
			require.NoError(t, store.Save(existing))
			prompter := &fakePrompter{interactive: true, line: "bob", secret: "pb"}

			res := runRegistry(t, store, prompter, nil, "pw\n", true, nil, "login", host, "-u", "admin", "--password-stdin")

			require.ErrorContains(t, res.err, "already logged in to a.example.com: run `cargueiro registry logout` first")
			require.Empty(t, res.calls, "nothing is checked against any registry")
			require.Empty(t, prompter.asked, "the user is not asked for anything")
			require.Empty(t, res.stdout)
			login, _ := saved(t, store)
			require.Equal(t, existing, login, "the saved login is untouched")
		})
	}
}

func TestLoginPasswordSources(t *testing.T) {
	tests := []struct {
		name     string
		prompter *fakePrompter
		env      map[string]string
		stdin    string
		args     []string
		wantUser string
		wantPass string
		wantWarn string
		asked    []string
	}{
		{
			name:     "password flag, with a warning",
			prompter: &fakePrompter{},
			args:     []string{"-u", "admin", "--password", "flag-pass"},
			wantUser: "admin", wantPass: "flag-pass",
			wantWarn: "--password on the command line is insecure",
		},
		{
			name:     "password from stdin, trailing newline removed",
			prompter: &fakePrompter{},
			stdin:    "stdin-pass\r\n",
			args:     []string{"-u", "admin", "--password-stdin"},
			wantUser: "admin", wantPass: "stdin-pass",
		},
		{
			name:     "username and password from the environment",
			prompter: &fakePrompter{},
			env:      map[string]string{"CARGUEIRO_USERNAME": "env-user", "CARGUEIRO_PASSWORD": "env-pass"},
			wantUser: "env-user", wantPass: "env-pass",
		},
		{
			name:     "username flag beats the environment",
			prompter: &fakePrompter{},
			env:      map[string]string{"CARGUEIRO_USERNAME": "env-user", "CARGUEIRO_PASSWORD": "env-pass"},
			args:     []string{"-u", "flag-user"},
			wantUser: "flag-user", wantPass: "env-pass",
		},
		{
			name:     "both asked for on a terminal",
			prompter: &fakePrompter{interactive: true, line: "typed-user", secret: "typed-pass"},
			wantUser: "typed-user", wantPass: "typed-pass",
			asked: []string{"Username: ", "Password: "},
		},
		{
			name:     "only the password asked for when the username is given",
			prompter: &fakePrompter{interactive: true, secret: "typed-pass"},
			args:     []string{"-u", "admin"},
			wantUser: "admin", wantPass: "typed-pass",
			asked: []string{"Password: "},
		},
		{
			name:     "nothing is asked when everything is given",
			prompter: &fakePrompter{interactive: true, line: "no", secret: "no"},
			env:      map[string]string{"CARGUEIRO_PASSWORD": "env-pass"},
			args:     []string{"-u", "admin"},
			wantUser: "admin", wantPass: "env-pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)

			res := runRegistry(t, store, tt.prompter, tt.env, tt.stdin, true, nil,
				append([]string{"login", "reg.example.com"}, tt.args...)...)

			require.NoError(t, res.err)
			want := auth.Login{Host: "reg.example.com", Username: tt.wantUser, Password: tt.wantPass}
			login, found := saved(t, store)
			require.True(t, found)
			require.Equal(t, want, login)
			require.Equal(t, tt.wantUser, res.calls[0].opts.Username)
			require.Equal(t, tt.wantPass, res.calls[0].opts.Password)
			if tt.wantWarn != "" {
				require.Contains(t, res.stderr, tt.wantWarn)
			} else {
				require.NotContains(t, res.stderr, "insecure")
			}
			require.Equal(t, tt.asked, tt.prompter.asked)
			requireNoSecret(t, res, tt.wantPass)
		})
	}
}

func TestLoginMissingInput(t *testing.T) {
	tests := []struct {
		name     string
		prompter *fakePrompter
		stdin    string
		args     []string
		wantErr  string
	}{
		{name: "no username without a terminal", prompter: &fakePrompter{}, args: []string{"--password", "p"}, wantErr: "a username is required"},
		{name: "no password without a terminal", prompter: &fakePrompter{}, args: []string{"-u", "admin"}, wantErr: "a password is required"},
		{name: "stdin carries the password so the username must be given", prompter: &fakePrompter{interactive: true},
			stdin: "pw\n", args: []string{"--password-stdin"}, wantErr: "because standard input carries the password"},
		{name: "empty password on stdin", prompter: &fakePrompter{}, stdin: "\n", args: []string{"-u", "admin", "--password-stdin"}, wantErr: "a password is required"},
		{name: "empty password typed", prompter: &fakePrompter{interactive: true, secret: ""}, args: []string{"-u", "admin"}, wantErr: "a password is required"},
		{name: "password flag and stdin together", prompter: &fakePrompter{}, args: []string{"-u", "admin", "--password", "p", "--password-stdin"}, wantErr: "none of the others can be"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)

			res := runRegistry(t, store, tt.prompter, nil, tt.stdin, true, nil,
				append([]string{"login", "reg.example.com"}, tt.args...)...)

			require.ErrorContains(t, res.err, tt.wantErr)
			require.Empty(t, res.calls, "nothing is checked against the registry")
			require.NoFileExists(t, store.Path(), "nothing is saved")
		})
	}

	t.Run("no host", func(t *testing.T) {
		res := runRegistry(t, newTestStore(t), &fakePrompter{}, nil, "", true, nil, "login")

		require.ErrorContains(t, res.err, "accepts 1 arg")
	})
}

func TestLoginFailureSavesNothing(t *testing.T) {
	store := newTestStore(t)
	failure := errors.New("401 Unauthorized")

	res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", false, failure,
		"login", "reg.example.com", "-u", "admin", "--password-stdin")

	require.ErrorIs(t, res.err, failure)
	require.ErrorContains(t, res.err, "login to reg.example.com failed")
	require.Empty(t, res.stdout)
	require.NoFileExists(t, store.Path(), "credentials that failed the check are never saved")
	requireNoSecret(t, res, "pw")
}

func TestLoginWarnsWhenTheCredentialsCannotBeChecked(t *testing.T) {
	store := newTestStore(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", false, nil,
		"login", "open.example.com", "-u", "admin", "--password-stdin")

	require.NoError(t, res.err)
	require.Contains(t, res.stderr, "open.example.com accepts anonymous requests, so these credentials could not be checked")
	_, found := saved(t, store)
	require.True(t, found, "they are saved anyway")
}

func writeCA(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(path, []byte(testCAPEM), 0o600))
	return path
}

func TestLoginReadsTheCAFileAndSavesItsContents(t *testing.T) {
	store := newTestStore(t)
	caFile := writeCA(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", true, nil,
		"login", "reg.example.com", "-u", "admin", "--password-stdin", "--ca-file", caFile)

	require.NoError(t, res.err)
	require.Equal(t, []byte(testCAPEM), res.calls[0].opts.CAPEM, "the check uses the certificate, not the path")
	login, _ := saved(t, store)
	require.Equal(t, testCAPEM, login.CAPEM)
	require.False(t, login.PlainHTTP)

	t.Run("the saved copy survives the file being deleted", func(t *testing.T) {
		require.NoError(t, os.Remove(caFile))

		login, found := saved(t, store)

		require.True(t, found)
		require.Equal(t, testCAPEM, login.CAPEM)
	})
}

func TestLoginWithoutTransportFlagsSavesNone(t *testing.T) {
	store := newTestStore(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", true, nil,
		"login", "reg.example.com", "-u", "admin", "--password-stdin")

	require.NoError(t, res.err)
	login, _ := saved(t, store)
	require.False(t, login.PlainHTTP)
	require.Empty(t, login.CAPEM)
}

func TestLoginRejectsABadCAFile(t *testing.T) {
	notPEM := filepath.Join(t.TempDir(), "junk.crt")
	require.NoError(t, os.WriteFile(notPEM, []byte("not a certificate"), 0o600))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing file", args: []string{"--ca-file", filepath.Join(t.TempDir(), "absent.crt")}, wantErr: "read CA file"},
		{name: "not PEM", args: []string{"--ca-file", notPEM}},
		{name: "with plain HTTP", args: []string{"--ca-file", "x", "--plain-http"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)

			res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", false, errors.New("rejected by the fake"),
				append([]string{"login", "reg.example.com", "-u", "admin", "--password-stdin"}, tt.args...)...)

			require.Error(t, res.err)
			if tt.wantErr != "" {
				require.ErrorContains(t, res.err, tt.wantErr)
			}
			require.NoFileExists(t, store.Path(), "nothing is saved")
		})
	}
}

func TestLoginFailsOnACorruptLoginFile(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(store.Path()), 0o700))
	require.NoError(t, os.WriteFile(store.Path(), []byte("{ not json"), 0o600))

	res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", true, nil,
		"login", "reg.example.com", "-u", "admin", "--password-stdin")

	require.ErrorContains(t, res.err, "registry logout", "it says how to recover")
	require.Empty(t, res.calls)
}

func TestLoginFailsWhenTheLoginFileCannotBeOpened(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: func() (*auth.Store, error) { return nil, errors.New("no config directory") },
		prompt:    &fakePrompter{},
	}
	called := false
	verify := func(context.Context, string, oci.ClientOptions) (bool, error) { called = true; return true, nil }
	root, err := newRootCmd(&out, &errOut, opts, dependencies{verifyLogin: verify})
	require.NoError(t, err)
	root.SetIn(strings.NewReader("pw\n"))
	root.SetArgs([]string{"registry", "login", "reg.example.com", "-u", "admin", "--password-stdin"})

	require.ErrorContains(t, root.Execute(), "no config directory")
	require.False(t, called, "fail before asking the user or the registry anything")
}

func TestRegistryWithoutASubcommandShowsHelp(t *testing.T) {
	res := runRegistry(t, newTestStore(t), &fakePrompter{}, nil, "", true, nil)

	require.NoError(t, res.err)
	require.Contains(t, res.stdout, "login")
	require.Contains(t, res.stdout, "logout")
}
