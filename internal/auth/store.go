// Package auth stores registry credentials in a file owned by cargueiro, in
// Docker's config.json format, optionally delegating to a native credential
// helper when one is installed.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	oras "oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	envConfigDir = "CARGUEIRO_CONFIG_DIR"
	appDir       = "cargueiro"
	fileName     = "auth.json"
)

var (
	// ErrInvalidCredential is returned for a credential that cannot be stored.
	ErrInvalidCredential = errors.New("invalid credential")
	// ErrInsecurePermissions is returned when the credentials file can be read
	// by users other than its owner.
	ErrInsecurePermissions = errors.New("credentials file is accessible to other users")
)

// Credential is a username and password for a registry.
type Credential struct {
	Username string
	Password string
}

// Options tune how a Store keeps credentials.
type Options struct {
	// UseNativeStore delegates to the platform credential helper (for example
	// docker-credential-secretservice) when one is installed. Without a
	// helper, credentials are kept in the file, base64-encoded.
	UseNativeStore bool
}

// Store keeps credentials per registry host.
type Store struct {
	path    string
	backend *credentials.DynamicStore
}

// DefaultPath returns where credentials are kept: $CARGUEIRO_CONFIG_DIR/auth.json
// when that variable is set, otherwise <user config dir>/cargueiro/auth.json.
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

// Open opens the store at path. The file and its directory are created, with
// modes 0600 and 0700, on the first Put.
func Open(path string, opts Options) (*Store, error) {
	backend, err := credentials.NewStore(path, credentials.StoreOptions{
		AllowPlaintextPut:        true,
		DetectDefaultNativeStore: opts.UseNativeStore,
	})
	if err != nil {
		return nil, fmt.Errorf("open credentials file %s: %w", path, err)
	}
	return &Store{path: path, backend: backend}, nil
}

// Path returns the credentials file location.
func (s *Store) Path() string { return s.path }

// Put saves cred for host and reports whether it ended up as plaintext
// (base64) in the file, as opposed to in a native credential helper.
func (s *Store) Put(ctx context.Context, host string, cred Credential) (plaintext bool, err error) {
	if err := validate(cred); err != nil {
		return false, err
	}

	key := canonical(host)
	if key == "" {
		return false, fmt.Errorf("%w: empty registry host", ErrInvalidCredential)
	}

	if err := s.backend.Put(ctx, key, oras.Credential{Username: cred.Username, Password: cred.Password}); err != nil {
		return false, fmt.Errorf("save credentials for %s: %w", key, err)
	}

	return s.storedInFile(key)
}

// Get returns the credential saved for host, and whether there is one.
func (s *Store) Get(ctx context.Context, host string) (Credential, bool, error) {
	key := canonical(host)
	if key == "" {
		return Credential{}, false, nil
	}

	got, err := s.backend.Get(ctx, key)
	if err != nil {
		return Credential{}, false, fmt.Errorf("read credentials for %s: %w", key, err)
	}
	if got == oras.EmptyCredential {
		return Credential{}, false, nil
	}
	return Credential{Username: got.Username, Password: got.Password}, true, nil
}

// Delete removes the credential saved for host and reports whether there was
// one. Deleting a host with no credential is not an error.
func (s *Store) Delete(ctx context.Context, host string) (bool, error) {
	_, existed, err := s.Get(ctx, host)
	if err != nil || !existed {
		return false, err
	}

	key := canonical(host)
	if err := s.backend.Delete(ctx, key); err != nil {
		return false, fmt.Errorf("delete credentials for %s: %w", key, err)
	}
	return true, nil
}

// CheckPermissions returns ErrInsecurePermissions when the credentials file
// exists and group or other users can access it. A missing file is fine.
func (s *Store) CheckPermissions() error {
	info, err := os.Stat(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat credentials file: %w", err)
	}

	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return fmt.Errorf("%w: %s has mode %04o, want 0600", ErrInsecurePermissions, s.path, mode)
	}
	return nil
}

// storedInFile reports whether key has a base64 entry in the credentials file.
func (s *Store) storedInFile(key string) (bool, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return false, fmt.Errorf("read credentials file: %w", err)
	}

	var content struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(data, &content); err != nil {
		return false, fmt.Errorf("parse credentials file: %w", err)
	}
	return content.Auths[key].Auth != "", nil
}

func validate(cred Credential) error {
	switch {
	case cred.Username == "":
		return fmt.Errorf("%w: empty username", ErrInvalidCredential)
	case cred.Password == "":
		return fmt.Errorf("%w: empty password", ErrInvalidCredential)
	case strings.Contains(cred.Username, ":"):
		// The file format joins username and password with ':'.
		return fmt.Errorf("%w: a username cannot contain ':'", ErrInvalidCredential)
	}
	return nil
}

// canonical is the key a host is stored under: lower case, without a trailing
// dot on the name. A port is part of the host, so registry.example.com and
// registry.example.com:443 are different keys.
func canonical(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))

	name, port := host, ""
	if i := strings.LastIndex(host, ":"); i >= 0 {
		name, port = host[:i], host[i:]
	}
	return strings.TrimSuffix(name, ".") + port
}
