//go:build e2e

// Package login holds the e2e tests of `cargueiro registry login` and
// `registry logout`, run against the real binary and the local registries
// (see `make registry-up`).
package login

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
)

const (
	fleetVersion = "110.0.3+up0.16.3"
	registryTag  = "110.0.3_up0.16.3"
)

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// transport is one way of reaching a local registry.
type transport struct {
	name    string
	address func(t *testing.T) string
	flags   func(t *testing.T) []string
}

func transports() []transport {
	return []transport{
		{name: "HTTP", address: harness.HTTPRegistry, flags: func(*testing.T) []string { return []string{"--plain-http"} }},
		{name: "TLS", address: harness.TLSRegistry, flags: func(t *testing.T) []string { return []string{"--ca-file", harness.RegistryCA(t)} }},
	}
}

// session is one user: a credentials directory that survives between commands.
type session struct{ configDir string }

func newSession(t *testing.T) session {
	t.Helper()

	return session{configDir: filepath.Join(t.TempDir(), "config")}
}

func (s session) authFile() string { return filepath.Join(s.configDir, "auth.json") }

func (s session) run(t *testing.T, stdin string, extraEnv []string, args ...string) harness.Result {
	t.Helper()

	return harness.CargueiroWith(t, harness.Options{
		Dir:   t.TempDir(),
		Stdin: stdin,
		Env:   append([]string{"CARGUEIRO_CONFIG_DIR=" + s.configDir}, extraEnv...),
	}, args...)
}

// login logs in as the e2e registry user, with the password on stdin.
func (s session) login(t *testing.T, address string, flags []string) harness.Result {
	t.Helper()

	args := append([]string{"registry", "login", address, "--username", harness.RegistryUser, "--password-stdin"}, flags...)
	return s.run(t, harness.RegistryPassword+"\n", nil, args...)
}

// push pushes one published fleet chart to address/path with no credentials
// given, so only saved ones can authenticate it.
func (s session) push(t *testing.T, address, path string, flags []string, extraEnv ...string) harness.Result {
	t.Helper()

	args := append([]string{"push", harness.FleetPublished(t, "fleet-crd", fleetVersion), "--registry", address + "/" + path}, flags...)
	return s.run(t, "", extraEnv, args...)
}

func repoPath() string { return fmt.Sprintf("e2e-login/%d", time.Now().UnixNano()) }

func requireNoPassword(t *testing.T, res harness.Result) {
	t.Helper()

	require.NotContains(t, res.Stdout, harness.RegistryPassword)
	require.NotContains(t, res.Stderr, harness.RegistryPassword)
}

func TestLoginThenPushWithoutCredentials(t *testing.T) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			s, address, path := newSession(t), tr.address(t), repoPath()

			login := s.login(t, address, tr.flags(t))

			requireNoPassword(t, login)
			require.Zero(t, login.ExitCode, login.Stderr)
			require.Equal(t, "Login succeeded for "+address+"\n", login.Stdout)
			require.Contains(t, login.Stderr, "stored unencrypted", "the plaintext storage is called out")

			push := s.push(t, address, path, tr.flags(t))

			require.Zero(t, push.ExitCode, push.Stderr)
			require.Equal(t, fmt.Sprintf("oci://%s/%s/fleet-crd:%s\n", address, path, registryTag), push.Stdout)
			pulled, pullRes := harness.HelmPull(t, address, path, "fleet-crd", fleetVersion, tr.flags(t))
			require.NotEmpty(t, pulled, pullRes.Stderr)
		})
	}
}

func TestNoLoginMeansNoCredentials(t *testing.T) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			s, address := newSession(t), tr.address(t)

			res := s.push(t, address, repoPath(), tr.flags(t))

			require.NotZero(t, res.ExitCode)
			require.Contains(t, res.Stderr, "hint: the registry requires credentials: run `cargueiro registry login "+address+"`")
		})
	}
}

func TestCredentialsFileIsPrivateAndHoldsNoPlainPassword(t *testing.T) {
	s, address := newSession(t), harness.HTTPRegistry(t)
	require.Zero(t, s.login(t, address, []string{"--plain-http"}).ExitCode)

	fileInfo, err := os.Stat(s.authFile())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
	dirInfo, err := os.Stat(s.configDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	content, err := os.ReadFile(s.authFile())
	require.NoError(t, err)
	require.Contains(t, string(content), address)
	require.NotContains(t, string(content), harness.RegistryPassword)
}

func TestLoginWithWrongPasswordSavesNothing(t *testing.T) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			s, address := newSession(t), tr.address(t)

			res := s.run(t, "not-the-password\n", nil,
				append([]string{"registry", "login", address, "-u", harness.RegistryUser, "--password-stdin"}, tr.flags(t)...)...)

			require.NotZero(t, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, "login to "+address+" failed")
			require.Contains(t, res.Stderr, `hint: the registry rejected the credentials of user "admin"`)
			require.NotContains(t, res.Stderr, "not-the-password")
			require.NoFileExists(t, s.authFile(), "credentials that fail the check are never saved")
		})
	}
}

func TestLoginTransportMismatches(t *testing.T) {
	tests := []struct {
		name     string
		address  func(t *testing.T) string
		flags    func(t *testing.T) []string
		wantHint string
	}{
		{
			name:     "TLS registry without its CA",
			address:  harness.TLSRegistry,
			flags:    func(*testing.T) []string { return nil },
			wantHint: "hint: the registry's certificate is not trusted: pass the CA that signed it with --ca-file",
		},
		{
			name:     "plain HTTP against the TLS registry",
			address:  harness.TLSRegistry,
			flags:    func(*testing.T) []string { return []string{"--plain-http"} },
			wantHint: "",
		},
		{
			name:     "HTTPS against the plain HTTP registry",
			address:  harness.HTTPRegistry,
			flags:    func(*testing.T) []string { return nil },
			wantHint: "hint: the registry answers plain HTTP: use --plain-http",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSession(t)

			res := s.login(t, tt.address(t), tt.flags(t))

			require.NotZero(t, res.ExitCode)
			require.Empty(t, res.Stdout)
			if tt.wantHint != "" {
				require.Contains(t, res.Stderr, tt.wantHint)
			}
			require.NoFileExists(t, s.authFile())
			requireNoPassword(t, res)
		})
	}
}

func TestLogoutThenPushFails(t *testing.T) {
	s, address, path := newSession(t), harness.HTTPRegistry(t), repoPath()
	flags := []string{"--plain-http"}
	require.Zero(t, s.login(t, address, flags).ExitCode)
	require.Zero(t, s.push(t, address, path, flags).ExitCode, "logged in: the push works")

	logout := s.run(t, "", nil, "registry", "logout", address)

	require.Zero(t, logout.ExitCode, logout.Stderr)
	require.Equal(t, "Removed credentials for "+address+"\n", logout.Stdout)
	after := s.push(t, address, path, flags)
	require.NotZero(t, after.ExitCode, "logged out: the push fails again")
	require.Contains(t, after.Stderr, "registry login "+address)

	t.Run("logging out twice is fine", func(t *testing.T) {
		again := s.run(t, "", nil, "registry", "logout", address)

		require.Zero(t, again.ExitCode, again.Stderr)
		require.Equal(t, "No credentials stored for "+address+"\n", again.Stdout)
	})

	t.Run("the file no longer holds the registry", func(t *testing.T) {
		content, err := os.ReadFile(s.authFile())
		require.NoError(t, err)
		require.NotContains(t, string(content), address)
	})
}

func TestLogoutNeverNeedsTheNetwork(t *testing.T) {
	s := newSession(t)

	res := s.run(t, "", []string{"HTTPS_PROXY=http://127.0.0.1:1", "HTTP_PROXY=http://127.0.0.1:1"},
		"registry", "logout", "registry.suse.com")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "No credentials stored for registry.suse.com\n", res.Stdout)
	require.NoFileExists(t, s.authFile(), "logging out must not create the file")
}

func TestSavedCredentialsBelongToTheirHostOnly(t *testing.T) {
	s, address := newSession(t), harness.HTTPRegistry(t)
	require.Zero(t, s.login(t, address, []string{"--plain-http"}).ExitCode)
	port := address[strings.LastIndex(address, ":"):]

	t.Run("another name for the same machine", func(t *testing.T) {
		res := s.push(t, "localhost"+port, repoPath(), []string{"--plain-http"})

		require.NotZero(t, res.ExitCode, "localhost is a different host from 127.0.0.1")
		require.Contains(t, res.Stderr, "registry login localhost"+port)
	})

	t.Run("another registry", func(t *testing.T) {
		tlsAddress := harness.TLSRegistry(t)

		res := s.push(t, tlsAddress, repoPath(), []string{"--ca-file", harness.RegistryCA(t)})

		require.NotZero(t, res.ExitCode)
		require.Contains(t, res.Stderr, "registry login "+tlsAddress)
	})
}

func TestExplicitCredentialsBeatSavedOnes(t *testing.T) {
	s, address, path := newSession(t), harness.HTTPRegistry(t), repoPath()
	flags := []string{"--plain-http"}
	require.Zero(t, s.login(t, address, flags).ExitCode)

	res := s.push(t, address, path, append(flags, "--username", harness.RegistryUser), "CARGUEIRO_PASSWORD=wrong-password")

	require.NotZero(t, res.ExitCode, "the saved, valid credentials must not rescue an explicit wrong one")
	require.Contains(t, res.Stderr, "401")
}

func TestLoginCredentialSources(t *testing.T) {
	address := harness.HTTPRegistry(t)
	flags := []string{"--plain-http"}

	t.Run("environment", func(t *testing.T) {
		s := newSession(t)

		res := s.run(t, "", []string{"CARGUEIRO_USERNAME=" + harness.RegistryUser, "CARGUEIRO_PASSWORD=" + harness.RegistryPassword},
			append([]string{"registry", "login", address}, flags...)...)

		require.Zero(t, res.ExitCode, res.Stderr)
		requireNoPassword(t, res)
		require.Zero(t, s.push(t, address, repoPath(), flags).ExitCode)
	})

	t.Run("password flag works and warns", func(t *testing.T) {
		s := newSession(t)

		res := s.run(t, "", nil, append([]string{"registry", "login", address, "-u", harness.RegistryUser, "--password", harness.RegistryPassword}, flags...)...)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Contains(t, res.Stderr, "--password on the command line is insecure")
		require.NotContains(t, res.Stderr, harness.RegistryPassword)
		require.Zero(t, s.push(t, address, repoPath(), flags).ExitCode)
	})

	t.Run("password flag and stdin together", func(t *testing.T) {
		s := newSession(t)

		res := s.run(t, "x\n", nil, append([]string{"registry", "login", address, "-u", "a", "--password", "b", "--password-stdin"}, flags...)...)

		require.NotZero(t, res.ExitCode)
		require.Contains(t, res.Stderr, "password")
		require.NoFileExists(t, s.authFile())
	})

	t.Run("no terminal and no username", func(t *testing.T) {
		s := newSession(t)

		res := s.run(t, "", nil, append([]string{"registry", "login", address, "--password", "x"}, flags...)...)

		require.NotZero(t, res.ExitCode)
		require.Contains(t, res.Stderr, "a username is required")
	})

	t.Run("logging in again replaces the credentials", func(t *testing.T) {
		s := newSession(t)
		require.Zero(t, s.login(t, address, flags).ExitCode)

		second := s.login(t, address, flags)

		require.Zero(t, second.ExitCode, second.Stderr)
	})
}

func TestLoginRefusesAnAddressWithAPath(t *testing.T) {
	s := newSession(t)

	res := s.login(t, harness.HTTPRegistry(t)+"/rancher/catalog", []string{"--plain-http"})

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "credentials belong to a host, not a path")
	require.NoFileExists(t, s.authFile())
}

func TestLoginToAnUnreachableRegistry(t *testing.T) {
	s := newSession(t)

	res := s.login(t, "127.0.0.1:1", []string{"--plain-http"})

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "hint: nothing accepted the connection: check the host and port in the registry address")
	require.NoFileExists(t, s.authFile())
}

func TestRegistryHelp(t *testing.T) {
	s := newSession(t)

	group := s.run(t, "", nil, "registry")
	login := s.run(t, "", nil, "registry", "login", "--help")

	require.Zero(t, group.ExitCode, group.Stderr)
	require.Contains(t, group.Stdout, "login")
	require.Contains(t, group.Stdout, "logout")
	require.Zero(t, login.ExitCode, login.Stderr)
	for _, flag := range []string{"--username", "--password-stdin", "--password", "--plain-http", "--ca-file"} {
		require.Contains(t, login.Stdout, flag)
	}
}
