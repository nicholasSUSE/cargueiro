//go:build e2e

// Package login holds the e2e tests of `cargueiro registry login` and
// `registry logout`, run against the real binary and the local registries
// (see `make registry-up`).
//
// Login is where a registry's credentials and how to reach it (plain HTTP, an
// extra CA) are saved, and `push` reuses both until logout.
package login

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

const (
	fleetVersion = "110.0.3+up0.16.3"
	registryTag  = "110.0.3_up0.16.3"
)

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

func repoPath() string { return fmt.Sprintf("e2e-login/%d", time.Now().UnixNano()) }

func archive(t *testing.T) string {
	t.Helper()

	return harness.FleetPublished(t, "fleet-crd", fleetVersion)
}

func requireNoPassword(t *testing.T, res harness.Result) {
	t.Helper()

	require.NotContains(t, res.Stdout, harness.RegistryPassword)
	require.NotContains(t, res.Stderr, harness.RegistryPassword)
}

func TestLoginThenPushWithoutCredentialsOrFlags(t *testing.T) {
	for _, tr := range harness.Transports() {
		t.Run(tr.Name, func(t *testing.T) {
			s, address, path := harness.NewSession(t), tr.Address(t), repoPath()

			login := s.Login(t, address, tr.LoginFlags(t))

			requireNoPassword(t, login)
			require.Zero(t, login.ExitCode, login.Stderr)
			require.Equal(t, "Login succeeded for "+address+"\n", login.Stdout)
			require.Contains(t, login.Stderr, "saved in plain text", "the plaintext storage is called out")

			push := s.Push(t, address+"/"+path, archive(t))

			require.Zero(t, push.ExitCode, push.Stderr)
			require.Equal(t, fmt.Sprintf("oci://%s/%s/fleet-crd:%s\n", address, path, registryTag), push.Stdout)
			pulled, pullRes := harness.HelmPull(t, address, path, "fleet-crd", fleetVersion, tr.HelmFlags(t))
			require.NotEmpty(t, pulled, pullRes.Stderr)
		})
	}
}

func TestNoLoginMeansNoWayToReachALocalRegistry(t *testing.T) {
	for _, tr := range harness.Transports() {
		t.Run(tr.Name, func(t *testing.T) {
			s, address := harness.NewSession(t), tr.Address(t)

			res := s.Push(t, address+"/"+repoPath(), archive(t))

			require.NotZero(t, res.ExitCode)
		})
	}
}

func TestLoginSavesWhatItNeedsToReuseTheRegistry(t *testing.T) {
	t.Run("plain HTTP", func(t *testing.T) {
		s, address := harness.NewSession(t), harness.HTTPRegistry(t)
		require.Zero(t, s.Login(t, address, []string{"--plain-http"}).ExitCode)

		content, err := os.ReadFile(s.LoginFile())
		require.NoError(t, err)

		require.Contains(t, string(content), `"host": "`+address+`"`)
		require.Contains(t, string(content), `"plainHTTP": true`)
		require.NotContains(t, string(content), "caCertificate")
	})

	t.Run("a CA certificate, by content", func(t *testing.T) {
		s, address := harness.NewSession(t), harness.TLSRegistry(t)
		ca, err := os.ReadFile(harness.RegistryCA(t))
		require.NoError(t, err)
		copied := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(copied, ca, 0o600))
		require.Zero(t, s.Login(t, address, []string{"--ca-file", copied}).ExitCode)

		require.NoError(t, os.Remove(copied))

		push := s.Push(t, address+"/"+repoPath(), archive(t))
		require.Zero(t, push.ExitCode, "the CA was saved at login, so the file is not needed any more: "+push.Stderr)
		content, err := os.ReadFile(s.LoginFile())
		require.NoError(t, err)
		require.Contains(t, string(content), "BEGIN CERTIFICATE")
		require.NotContains(t, string(content), "plainHTTP")
	})
}

func TestTheLoginFileIsPrivateAndTheOnlyFile(t *testing.T) {
	s, address := harness.NewSession(t), harness.HTTPRegistry(t)
	require.Zero(t, s.Login(t, address, []string{"--plain-http"}).ExitCode)

	info, err := os.Stat(s.LoginFile())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	dirInfo, err := os.Stat(s.ConfigDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
	entries, err := os.ReadDir(s.ConfigDir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "one login, one file, nothing else left behind")
	content, err := os.ReadFile(s.LoginFile())
	require.NoError(t, err)
	require.Contains(t, string(content), `"password": "`+harness.RegistryPassword+`"`, "stored in plain text, as the warning says")
}

func TestLoginWithWrongPasswordSavesNothing(t *testing.T) {
	for _, tr := range harness.Transports() {
		t.Run(tr.Name, func(t *testing.T) {
			s, address := harness.NewSession(t), tr.Address(t)

			res := s.Run(t, "not-the-password\n", nil,
				append([]string{"registry", "login", address, "-u", harness.RegistryUser, "--password-stdin"}, tr.LoginFlags(t)...)...)

			require.NotZero(t, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, "login to "+address+" failed")
			require.NotContains(t, res.Stderr, "not-the-password")
			require.NoFileExists(t, s.LoginFile(), "credentials that fail the check are never saved")
		})
	}
}

func TestLoginTransportMismatches(t *testing.T) {
	tests := []struct {
		name    string
		address func(t *testing.T) string
		flags   func(t *testing.T) []string
	}{
		{
			name:    "TLS registry without its CA",
			address: harness.TLSRegistry,
			flags:   func(*testing.T) []string { return nil },
		},
		{
			name:    "plain HTTP against the TLS registry",
			address: harness.TLSRegistry,
			flags:   func(*testing.T) []string { return []string{"--plain-http"} },
		},
		{
			name:    "HTTPS against the plain HTTP registry",
			address: harness.HTTPRegistry,
			flags:   func(*testing.T) []string { return nil },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := harness.NewSession(t)

			res := s.Login(t, tt.address(t), tt.flags(t))

			require.NotZero(t, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.NoFileExists(t, s.LoginFile())
			requireNoPassword(t, res)
		})
	}
}

func TestLoginWithABadCA(t *testing.T) {
	address := harness.TLSRegistry(t)
	junk := filepath.Join(t.TempDir(), "junk.crt")
	require.NoError(t, os.WriteFile(junk, []byte("not a certificate"), 0o600))
	good := harness.RegistryCA(t)

	tests := []struct {
		name    string
		flags   []string
		wantErr string
	}{
		{name: "missing file", flags: []string{"--ca-file", filepath.Join(t.TempDir(), "absent.crt")}, wantErr: "read CA file"},
		{name: "not PEM", flags: []string{"--ca-file", junk}, wantErr: "no PEM certificate"},
		{name: "with plain HTTP", flags: []string{"--ca-file", good, "--plain-http"}, wantErr: "plain HTTP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := harness.NewSession(t)

			res := s.Login(t, address, tt.flags)

			require.NotZero(t, res.ExitCode)
			require.Contains(t, res.Stderr, tt.wantErr)
			require.NoFileExists(t, s.LoginFile())
		})
	}
}

func TestALoginWhileLoggedInAsksToLogOutFirst(t *testing.T) {
	s := harness.NewSession(t)
	httpAddress := harness.HTTPRegistry(t)
	require.Zero(t, s.Login(t, httpAddress, []string{"--plain-http"}).ExitCode)
	before, err := os.ReadFile(s.LoginFile())
	require.NoError(t, err)

	t.Run("the same registry", func(t *testing.T) {
		res := s.Login(t, httpAddress, []string{"--plain-http"})

		require.NotZero(t, res.ExitCode)
		require.Empty(t, res.Stdout)
		require.Contains(t, res.Stderr, "already logged in to "+httpAddress+": run `cargueiro registry logout` first")
	})

	t.Run("another registry", func(t *testing.T) {
		res := s.Login(t, harness.TLSRegistry(t), []string{"--ca-file", harness.RegistryCA(t)})

		require.NotZero(t, res.ExitCode)
		require.Contains(t, res.Stderr, "already logged in to "+httpAddress+": run `cargueiro registry logout` first")
	})

	after, err := os.ReadFile(s.LoginFile())
	require.NoError(t, err)
	require.Equal(t, string(before), string(after), "the saved login is untouched")
	require.Zero(t, s.Push(t, httpAddress+"/"+repoPath(), archive(t)).ExitCode, "and still works")

	t.Run("after a logout the other registry can be logged in to", func(t *testing.T) {
		require.Zero(t, s.Run(t, "", nil, "registry", "logout").ExitCode)
		tlsAddress := harness.TLSRegistry(t)

		login := s.Login(t, tlsAddress, []string{"--ca-file", harness.RegistryCA(t)})

		require.Zero(t, login.ExitCode, login.Stderr)
		require.Zero(t, s.Push(t, tlsAddress+"/"+repoPath(), archive(t)).ExitCode)
		require.NotZero(t, s.Push(t, httpAddress+"/"+repoPath(), archive(t)).ExitCode, "the first registry is not remembered")
	})
}

func TestLogoutForgetsEverything(t *testing.T) {
	s, address, path := harness.NewSession(t), harness.HTTPRegistry(t), repoPath()
	require.Zero(t, s.Login(t, address, []string{"--plain-http"}).ExitCode)
	require.Zero(t, s.Push(t, address+"/"+path, archive(t)).ExitCode, "logged in: the push works")

	logout := s.Run(t, "", nil, "registry", "logout")

	require.Zero(t, logout.ExitCode, logout.Stderr)
	require.Equal(t, "Removed the saved login for "+address+"\n", logout.Stdout)
	after := s.Push(t, address+"/"+path, archive(t))
	require.NotZero(t, after.ExitCode, "logged out: the push fails again")
	require.Contains(t, after.Stderr, "HTTP response to HTTPS client", "the plain-HTTP setting went with the login")

	t.Run("the login file is gone", func(t *testing.T) {
		require.NoFileExists(t, s.LoginFile())
	})

	t.Run("logging out twice is fine", func(t *testing.T) {
		again := s.Run(t, "", nil, "registry", "logout")

		require.Zero(t, again.ExitCode, again.Stderr)
		require.Equal(t, "No saved login\n", again.Stdout)
	})
}

func TestLogoutNeverNeedsTheNetwork(t *testing.T) {
	s := harness.NewSession(t)

	res := s.Run(t, "", []string{"HTTPS_PROXY=http://127.0.0.1:1", "HTTP_PROXY=http://127.0.0.1:1"},
		"registry", "logout")

	require.Zero(t, res.ExitCode, res.Stderr)
	require.Equal(t, "No saved login\n", res.Stdout)
	require.NoFileExists(t, s.LoginFile(), "logging out must not create the file")
}

func TestExplicitCredentialsBeatSavedOnes(t *testing.T) {
	s, address, path := harness.NewSession(t), harness.HTTPRegistry(t), repoPath()
	require.Zero(t, s.Login(t, address, []string{"--plain-http"}).ExitCode)

	res := s.Run(t, "", []string{"CARGUEIRO_PASSWORD=wrong-password"},
		"push", archive(t), "--registry", address+"/"+path, "--username", harness.RegistryUser)

	require.NotZero(t, res.ExitCode, "the saved, valid credentials must not rescue an explicit wrong one")
	require.Contains(t, res.Stderr, "401")
}

func TestLoginCredentialSources(t *testing.T) {
	address := harness.HTTPRegistry(t)
	flags := []string{"--plain-http"}

	t.Run("environment", func(t *testing.T) {
		s := harness.NewSession(t)

		res := s.Run(t, "", []string{"CARGUEIRO_USERNAME=" + harness.RegistryUser, "CARGUEIRO_PASSWORD=" + harness.RegistryPassword},
			append([]string{"registry", "login", address}, flags...)...)

		require.Zero(t, res.ExitCode, res.Stderr)
		requireNoPassword(t, res)
		require.Zero(t, s.Push(t, address+"/"+repoPath(), archive(t)).ExitCode)
	})

	t.Run("password flag works and warns", func(t *testing.T) {
		s := harness.NewSession(t)

		res := s.Run(t, "", nil, append([]string{"registry", "login", address, "-u", harness.RegistryUser, "--password", harness.RegistryPassword}, flags...)...)

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Contains(t, res.Stderr, "--password on the command line is insecure")
		require.NotContains(t, res.Stderr, harness.RegistryPassword)
		require.Zero(t, s.Push(t, address+"/"+repoPath(), archive(t)).ExitCode)
	})

	t.Run("password flag and stdin together", func(t *testing.T) {
		s := harness.NewSession(t)

		res := s.Run(t, "x\n", nil, append([]string{"registry", "login", address, "-u", "a", "--password", "b", "--password-stdin"}, flags...)...)

		require.NotZero(t, res.ExitCode)
		require.Contains(t, res.Stderr, "password")
		require.NoFileExists(t, s.LoginFile())
	})

	t.Run("no terminal and no username", func(t *testing.T) {
		s := harness.NewSession(t)

		res := s.Run(t, "", nil, append([]string{"registry", "login", address, "--password", "x"}, flags...)...)

		require.NotZero(t, res.ExitCode)
		require.Contains(t, res.Stderr, "a username is required")
	})
}

func TestLoginRefusesAnAddressWithAPath(t *testing.T) {
	s := harness.NewSession(t)

	res := s.Login(t, harness.HTTPRegistry(t)+"/rancher/catalog", []string{"--plain-http"})

	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, "credentials belong to a host, not a path")
	require.NoFileExists(t, s.LoginFile())
}

func TestLoginToAnUnreachableRegistry(t *testing.T) {
	s := harness.NewSession(t)

	res := s.Login(t, "127.0.0.1:1", []string{"--plain-http"})

	require.NotZero(t, res.ExitCode)
	require.NoFileExists(t, s.LoginFile())
}
