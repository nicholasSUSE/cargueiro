// Package auth keeps the one registry login cargueiro remembers: which
// registry, who to authenticate as, and how to reach it. There is only ever
// one; logging in again means logging out first.
package auth

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	envConfigDir = "CARGUEIRO_CONFIG_DIR"
	appDir       = "cargueiro"
	fileName     = "login.json"
)

// ErrInsecurePermissions is returned when the login file can be read by
// users other than its owner.
var ErrInsecurePermissions = errors.New("login file is accessible to other users")

// Login is the saved registry login.
type Login struct {
	// Host is the registry host, with its port if it has one.
	Host     string `json:"host"`
	Username string `json:"username"`
	// Password is kept in plain text in the login file, which only its owner
	// can read.
	Password string `json:"password"`
	// PlainHTTP reaches the registry over HTTP instead of HTTPS.
	PlainHTTP bool `json:"plainHTTP,omitempty"`
	// CAPEM holds extra CA certificates to trust, in PEM. It is the contents
	// of the file given at login, so later changes to that file do not matter.
	CAPEM string `json:"caCertificate,omitempty"`
}

// Matches reports whether the login is for host. Host names are compared
// without regard to case, and the port is part of the host.
func (l Login) Matches(host string) bool {
	return l.Host != "" && strings.EqualFold(l.Host, host)
}

func (l Login) validate() error {
	switch {
	case l.Host == "":
		return errors.New("invalid login: empty registry host")
	case l.Username == "":
		return errors.New("invalid login: empty username")
	case l.Password == "":
		return errors.New("invalid login: empty password")
	case l.PlainHTTP && l.CAPEM != "":
		return errors.New("invalid login: a CA certificate cannot be used with plain HTTP")
	}

	if l.CAPEM != "" {
		block, _ := pem.Decode([]byte(l.CAPEM))
		if block == nil || block.Type != "CERTIFICATE" {
			return errors.New("invalid login: the CA certificate is not PEM")
		}
	}
	return nil
}

// Store is the file holding the login.
type Store struct{ path string }

// DefaultPath returns where the login is kept: $CARGUEIRO_CONFIG_DIR/login.json
// when that variable is set, otherwise <user config dir>/cargueiro/login.json.
func DefaultPath() (string, error) {
	if dir := os.Getenv(envConfigDir); dir != "" {
		return filepath.Join(dir, fileName), nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find the user config directory (set %s to choose one): %w", envConfigDir, err)
	}
	return filepath.Join(base, appDir, fileName), nil
}

// Open returns the store at path. Nothing is read or created until it is used.
func Open(path string) *Store { return &Store{path: path} }

// Path returns the login file location.
func (s *Store) Path() string { return s.path }

// Load returns the saved login, and whether there is one.
func (s *Store) Load() (Login, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return Login{}, false, nil
	}
	if err != nil {
		return Login{}, false, fmt.Errorf("read login file: %w", err)
	}

	var login Login
	if err := json.Unmarshal(data, &login); err != nil {
		return Login{}, false, fmt.Errorf("parse login file %s (remove it with `cargueiro registry logout`): %w", s.path, err)
	}
	if err := login.validate(); err != nil {
		return Login{}, false, fmt.Errorf("login file %s (remove it with `cargueiro registry logout`): %w", s.path, err)
	}
	return login, true, nil
}

// Save writes the login, replacing any earlier one. The file is created
// atomically with mode 0600 in a directory with mode 0700.
func (s *Store) Save(login Login) error {
	if err := login.validate(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(login, "", "\t")
	if err != nil {
		return fmt.Errorf("encode login: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, fileName+".*")
	if err != nil {
		return fmt.Errorf("save login: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // a no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("save login: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save login: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("save login: %w", err)
	}
	return nil
}

// Delete removes the saved login and reports whether there was one. It does
// not read the file, so it also removes one that is corrupt.
func (s *Store) Delete() (bool, error) {
	err := os.Remove(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("remove login file: %w", err)
	}
	return true, nil
}

// CheckPermissions returns ErrInsecurePermissions when the login file exists
// and group or other users can access it. A missing file is fine.
func (s *Store) CheckPermissions() error {
	info, err := os.Stat(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat login file: %w", err)
	}

	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return fmt.Errorf("%w: %s has mode %04o, want 0600", ErrInsecurePermissions, s.path, mode)
	}
	return nil
}
