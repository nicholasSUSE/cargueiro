package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

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

// runRegistry executes a `registry ...` command with fakes for the credential
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

func saved(t *testing.T, store *auth.Store, host string) (auth.Credential, bool) {
	t.Helper()

	cred, found, err := store.Get(context.Background(), host)
	require.NoError(t, err)
	return cred, found
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

func TestLoginSavesVerifiedCredentials(t *testing.T) {
	store := newTestStore(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "s3cret-pass\n", true, nil,
		"login", "reg.example.com:5000", "--username", "admin", "--password-stdin", "--plain-http")

	require.NoError(t, res.err)
	require.Equal(t, "Login succeeded for reg.example.com:5000\n", res.stdout)
	require.Equal(t, []verifyCall{{
		host: "reg.example.com:5000",
		opts: oci.ClientOptions{Username: "admin", Password: "s3cret-pass", PlainHTTP: true},
	}}, res.calls, "the credentials are checked against the registry first")
	cred, found := saved(t, store, "reg.example.com:5000")
	require.True(t, found)
	require.Equal(t, auth.Credential{Username: "admin", Password: "s3cret-pass"}, cred)
	require.Contains(t, res.stderr, "stored unencrypted", "plaintext storage is called out")
	require.Contains(t, res.stderr, store.Path())
	requireNoSecret(t, res, "s3cret-pass")
}

func TestLoginPassesTheCAFile(t *testing.T) {
	res := runRegistry(t, newTestStore(t), &fakePrompter{}, nil, "pw\n", true, nil,
		"login", "reg.example.com", "-u", "admin", "--password-stdin", "--ca-file", "ca.crt")

	require.NoError(t, res.err)
	require.Equal(t, "ca.crt", res.calls[0].opts.CAFile)
}

func TestLoginHostForms(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantHost string
		wantKey  string
	}{
		{name: "plain host", arg: "reg.example.com", wantHost: "reg.example.com", wantKey: "reg.example.com"},
		{name: "scheme is ignored", arg: "https://reg.example.com", wantHost: "reg.example.com", wantKey: "reg.example.com"},
		{name: "oci scheme and slash", arg: "oci://reg.example.com/", wantHost: "reg.example.com", wantKey: "reg.example.com"},
		{name: "host is saved case-insensitively", arg: "Reg.Example.COM:5000", wantHost: "Reg.Example.COM:5000", wantKey: "reg.example.com:5000"},
		{name: "a protected SUSE host can be logged in to", arg: "registry.suse.com", wantHost: "registry.suse.com", wantKey: "registry.suse.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)

			res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", true, nil,
				"login", tt.arg, "-u", "admin", "--password-stdin")

			require.NoError(t, res.err)
			require.Equal(t, tt.wantHost, res.calls[0].host)
			_, found := saved(t, store, tt.wantKey)
			require.True(t, found)
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
			require.Equal(t, auth.Credential{Username: tt.wantUser, Password: tt.wantPass}, credentialOf(res.calls[0].opts))
			cred, found := saved(t, store, "reg.example.com")
			require.True(t, found)
			require.Equal(t, auth.Credential{Username: tt.wantUser, Password: tt.wantPass}, cred)
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

func credentialOf(o oci.ClientOptions) auth.Credential {
	return auth.Credential{Username: o.Username, Password: o.Password}
}

func TestLoginMissingInput(t *testing.T) {
	tests := []struct {
		name     string
		prompter *fakePrompter
		env      map[string]string
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
		{name: "no host", prompter: &fakePrompter{}, args: nil, wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			args := append([]string{"login", "reg.example.com"}, tt.args...)
			if tt.name == "no host" {
				args = []string{"login"}
				tt.wantErr = "accepts 1 arg"
			}

			res := runRegistry(t, store, tt.prompter, tt.env, tt.stdin, true, nil, args...)

			require.ErrorContains(t, res.err, tt.wantErr)
			require.Empty(t, res.calls, "nothing is checked against the registry")
			require.NoFileExists(t, store.Path(), "nothing is saved")
		})
	}
}

func TestLoginFailureSavesNothingAndHints(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantHint string
	}{
		{
			name:     "credentials rejected",
			err:      &oci.RegistryError{Kind: oci.KindAuthRejected, Err: errors.New("401 Unauthorized")},
			wantHint: `hint: the registry rejected the credentials of user "admin"`,
		},
		{
			name:     "TLS registry without its CA",
			err:      &oci.RegistryError{Kind: oci.KindUntrustedCertificate, Err: errors.New("x509: unknown authority")},
			wantHint: "hint: the registry's certificate is not trusted: pass the CA that signed it with --ca-file",
		},
		{
			name:     "unreachable names the address, not a flag that does not exist",
			err:      &oci.RegistryError{Kind: oci.KindUnreachable, Err: errors.New("connection refused")},
			wantHint: "check the host and port in the registry address",
		},
		{
			name: "no hint for an unknown failure",
			err:  errors.New("boom"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)

			res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", false, tt.err,
				"login", "reg.example.com", "-u", "admin", "--password-stdin")

			require.ErrorIs(t, res.err, tt.err)
			require.ErrorContains(t, res.err, "login to reg.example.com failed")
			if tt.wantHint != "" {
				require.ErrorContains(t, res.err, tt.wantHint)
			} else {
				require.NotContains(t, res.err.Error(), "hint:")
			}
			require.Empty(t, res.stdout)
			require.NoFileExists(t, store.Path(), "credentials that failed the check are never saved")
			requireNoSecret(t, res, "pw")
		})
	}
}

func TestLoginWarnsWhenTheCredentialsCannotBeChecked(t *testing.T) {
	store := newTestStore(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", false, nil,
		"login", "open.example.com", "-u", "admin", "--password-stdin")

	require.NoError(t, res.err)
	require.Contains(t, res.stderr, "open.example.com accepts anonymous requests, so these credentials could not be checked")
	_, found := saved(t, store, "open.example.com")
	require.True(t, found, "they are saved anyway")
}

func TestLoginReplacesExistingCredentials(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Put(context.Background(), "reg.example.com", auth.Credential{Username: "old", Password: "old-pass"})
	require.NoError(t, err)

	res := runRegistry(t, store, &fakePrompter{}, nil, "new-pass\n", true, nil,
		"login", "reg.example.com", "-u", "new", "--password-stdin")

	require.NoError(t, res.err)
	cred, _ := saved(t, store, "reg.example.com")
	require.Equal(t, auth.Credential{Username: "new", Password: "new-pass"}, cred)
}

func TestLoginTightensALooseCredentialsFile(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Put(context.Background(), "other.example.com", auth.Credential{Username: "u", Password: "p"})
	require.NoError(t, err)
	require.NoError(t, os.Chmod(store.Path(), 0o644))

	res := runRegistry(t, store, &fakePrompter{}, nil, "pw\n", true, nil,
		"login", "reg.example.com", "-u", "admin", "--password-stdin")

	require.NoError(t, res.err)
	require.Contains(t, res.stderr, "accessible to other users")
	require.Contains(t, res.stderr, "now restricted")
	require.NoError(t, store.CheckPermissions())
}

func TestLoginFailsOnACorruptCredentialsFile(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: func() (*auth.Store, error) { return nil, errors.New("credentials file is corrupt") },
		prompt:    &fakePrompter{},
	}
	called := false
	verify := func(context.Context, string, oci.ClientOptions) (bool, error) { called = true; return true, nil }
	root, err := newRootCmd(&out, &errOut, opts, dependencies{verifyLogin: verify})
	require.NoError(t, err)
	root.SetIn(strings.NewReader("pw\n"))
	root.SetArgs([]string{"registry", "login", "reg.example.com", "-u", "admin", "--password-stdin"})

	require.ErrorContains(t, root.Execute(), "credentials file is corrupt")
	require.False(t, called, "fail before asking the user or the registry anything")
}

func TestRegistryWithoutASubcommandShowsHelp(t *testing.T) {
	res := runRegistry(t, newTestStore(t), &fakePrompter{}, nil, "", true, nil)

	require.NoError(t, res.err)
	require.Contains(t, res.stdout, "login")
	require.Contains(t, res.stdout, "logout")
}
