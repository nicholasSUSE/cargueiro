//go:build e2e

package harness

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Transport is one way of reaching a local e2e registry.
type Transport struct {
	Name    string
	Address func(t *testing.T) string
	// LoginFlags are the flags `cargueiro registry login` needs for it.
	LoginFlags func(t *testing.T) []string
	// HelmFlags are the flags the helm CLI needs for it.
	HelmFlags func(t *testing.T) []string
}

// Transports returns the plain-HTTP and the TLS e2e registries.
func Transports() []Transport {
	return []Transport{
		{
			Name:       "HTTP",
			Address:    HTTPRegistry,
			LoginFlags: func(*testing.T) []string { return []string{"--plain-http"} },
			HelmFlags:  func(*testing.T) []string { return []string{"--plain-http"} },
		},
		{
			Name:       "TLS",
			Address:    TLSRegistry,
			LoginFlags: func(t *testing.T) []string { return []string{"--ca-file", RegistryCA(t)} },
			HelmFlags:  func(t *testing.T) []string { return []string{"--ca-file", RegistryCA(t)} },
		},
	}
}

// Session is one user: a credentials directory that lives between commands,
// so that the login `registry login` saves is there for the next command.
type Session struct{ ConfigDir string }

// NewSession returns a session with an empty credentials directory.
func NewSession(t *testing.T) Session {
	t.Helper()

	return Session{ConfigDir: filepath.Join(t.TempDir(), "config")}
}

// LoginFile is where the session's login is saved.
func (s Session) LoginFile() string { return filepath.Join(s.ConfigDir, "login.json") }

// Run runs cargueiro as this session.
func (s Session) Run(t *testing.T, stdin string, extraEnv []string, args ...string) Result {
	t.Helper()

	return CargueiroWith(t, Options{
		Dir:   t.TempDir(),
		Stdin: stdin,
		Env:   append([]string{"CARGUEIRO_CONFIG_DIR=" + s.ConfigDir}, extraEnv...),
	}, args...)
}

// Login logs in to address as the e2e registry user, with the password on
// stdin and the given flags.
func (s Session) Login(t *testing.T, address string, flags []string) Result {
	t.Helper()

	args := append([]string{"registry", "login", address, "--username", RegistryUser, "--password-stdin"}, flags...)
	return s.Run(t, RegistryPassword+"\n", nil, args...)
}

// LoginVia logs in over tr, failing the test if that does not work, and
// returns the registry address.
func (s Session) LoginVia(t *testing.T, tr Transport) string {
	t.Helper()

	address := tr.Address(t)
	res := s.Login(t, address, tr.LoginFlags(t))
	require.Zero(t, res.ExitCode, res.Stderr)
	return address
}

// Push pushes archives to registry. No credentials and no transport settings
// are given: only what was saved by Login can authenticate or reach it.
func (s Session) Push(t *testing.T, registry string, archives ...string) Result {
	t.Helper()

	return s.Run(t, "", nil, append(append([]string{"push"}, archives...), "--registry", registry)...)
}
