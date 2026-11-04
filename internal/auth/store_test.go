package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func openStore(t *testing.T) *Store {
	t.Helper()

	s, err := Open(filepath.Join(t.TempDir(), "cfg", "auth.json"), Options{})
	require.NoError(t, err)
	return s
}

func TestDefaultPath(t *testing.T) {
	t.Run("explicit directory wins", func(t *testing.T) {
		t.Setenv("CARGUEIRO_CONFIG_DIR", "/custom/dir")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")

		got, err := DefaultPath()

		require.NoError(t, err)
		require.Equal(t, "/custom/dir/auth.json", got)
	})

	t.Run("XDG config home", func(t *testing.T) {
		t.Setenv("CARGUEIRO_CONFIG_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")

		got, err := DefaultPath()

		require.NoError(t, err)
		require.Equal(t, "/xdg/cargueiro/auth.json", got)
	})

	t.Run("home directory fallback", func(t *testing.T) {
		t.Setenv("CARGUEIRO_CONFIG_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "/home/someone")

		got, err := DefaultPath()

		require.NoError(t, err)
		require.Equal(t, "/home/someone/.config/cargueiro/auth.json", got)
	})
}

func TestPutGetDelete(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	cred := Credential{Username: "admin", Password: "s3cret: with spaces & ünïcode"}

	_, found, err := s.Get(ctx, "reg.example.com")
	require.NoError(t, err)
	require.False(t, found, "nothing is stored at first")

	_, err = s.Put(ctx, "reg.example.com", cred)
	require.NoError(t, err)

	got, found, err := s.Get(ctx, "reg.example.com")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, cred, got, "the password survives unchanged, colons and all")

	existed, err := s.Delete(ctx, "reg.example.com")
	require.NoError(t, err)
	require.True(t, existed)

	_, found, err = s.Get(ctx, "reg.example.com")
	require.NoError(t, err)
	require.False(t, found)
}

func TestDeleteMissingIsNotAnError(t *testing.T) {
	existed, err := openStore(t).Delete(context.Background(), "nowhere.example.com")

	require.NoError(t, err)
	require.False(t, existed)
}

func TestHostsAreIndependent(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	_, err := s.Put(ctx, "a.example.com", Credential{Username: "alice", Password: "pa"})
	require.NoError(t, err)
	_, err = s.Put(ctx, "b.example.com", Credential{Username: "bob", Password: "pb"})
	require.NoError(t, err)

	_, found, err := s.Get(ctx, "c.example.com")
	require.NoError(t, err)
	require.False(t, found, "credentials are never offered to another host")

	_, err = s.Delete(ctx, "a.example.com")
	require.NoError(t, err)
	got, found, err := s.Get(ctx, "b.example.com")
	require.NoError(t, err)
	require.True(t, found, "deleting one host keeps the others")
	require.Equal(t, "bob", got.Username)
}

func TestHostKeys(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	_, err := s.Put(ctx, "Registry.Example.COM:5000", Credential{Username: "u", Password: "p"})
	require.NoError(t, err)

	tests := []struct {
		name  string
		host  string
		found bool
	}{
		{name: "same host", host: "registry.example.com:5000", found: true},
		{name: "different case", host: "REGISTRY.example.com:5000", found: true},
		{name: "trailing dot", host: "registry.example.com.:5000", found: true},
		{name: "surrounding space", host: " registry.example.com:5000 ", found: true},
		{name: "port is part of the host", host: "registry.example.com", found: false},
		{name: "another port", host: "registry.example.com:443", found: false},
		{name: "empty host", host: "", found: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found, err := s.Get(ctx, tt.host)

			require.NoError(t, err)
			require.Equal(t, tt.found, found)
		})
	}
}

func TestPersistsAcrossOpens(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "auth.json")
	first, err := Open(path, Options{})
	require.NoError(t, err)
	_, err = first.Put(ctx, "reg.example.com", Credential{Username: "u", Password: "p"})
	require.NoError(t, err)

	second, err := Open(path, Options{})
	require.NoError(t, err)

	got, found, err := second.Get(ctx, "reg.example.com")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, Credential{Username: "u", Password: "p"}, got)
}

func TestFileAndDirectoryModes(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)

	_, err := s.Put(ctx, "reg.example.com", Credential{Username: "u", Password: "p"})
	require.NoError(t, err)

	fileInfo, err := os.Stat(s.Path())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm(), "credentials file")
	dirInfo, err := os.Stat(filepath.Dir(s.Path()))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "its directory")

	t.Run("a loose file is tightened on the next write", func(t *testing.T) {
		require.NoError(t, os.Chmod(s.Path(), 0o644))

		_, err := s.Put(ctx, "other.example.com", Credential{Username: "u", Password: "p"})
		require.NoError(t, err)

		info, err := os.Stat(s.Path())
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})
}

func TestPutReportsPlaintext(t *testing.T) {
	s := openStore(t)

	plaintext, err := s.Put(context.Background(), "reg.example.com", Credential{Username: "u", Password: "p"})

	require.NoError(t, err)
	require.True(t, plaintext, "without a native helper the password is only base64 in the file")
}

func TestCheckPermissions(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	require.NoError(t, s.CheckPermissions(), "a missing file is fine")

	_, err := s.Put(ctx, "reg.example.com", Credential{Username: "u", Password: "p"})
	require.NoError(t, err)
	require.NoError(t, s.CheckPermissions())

	for _, mode := range []os.FileMode{0o640, 0o604, 0o644, 0o666, 0o660} {
		require.NoError(t, os.Chmod(s.Path(), mode))
		require.ErrorIs(t, s.CheckPermissions(), ErrInsecurePermissions, "mode %04o", mode)
	}

	require.NoError(t, os.Chmod(s.Path(), 0o400))
	require.NoError(t, s.CheckPermissions(), "read-only for the owner is not a leak")
}

func TestPutRejectsBadCredentials(t *testing.T) {
	tests := []struct {
		name string
		host string
		cred Credential
	}{
		{name: "empty username", host: "h", cred: Credential{Password: "p"}},
		{name: "empty password", host: "h", cred: Credential{Username: "u"}},
		{name: "colon in username", host: "h", cred: Credential{Username: "a:b", Password: "p"}},
		{name: "empty host", host: "", cred: Credential{Username: "u", Password: "p"}},
		{name: "blank host", host: "  ", cred: Credential{Username: "u", Password: "p"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := openStore(t)

			_, err := s.Put(context.Background(), tt.host, tt.cred)

			require.ErrorIs(t, err, ErrInvalidCredential)
			require.NoFileExists(t, s.Path(), "a rejected credential must leave no file behind")
		})
	}
}

func TestOpenRejectsACorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	require.NoError(t, os.WriteFile(path, []byte("{ this is not json"), 0o600))

	_, err := Open(path, Options{})

	require.Error(t, err)
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, "{ this is not json", string(content), "a corrupt file is never overwritten")
}

func TestPasswordIsNotStoredAsPlainText(t *testing.T) {
	s := openStore(t)
	_, err := s.Put(context.Background(), "reg.example.com", Credential{Username: "admin", Password: "hunter2-hunter2"})
	require.NoError(t, err)

	content, err := os.ReadFile(s.Path())
	require.NoError(t, err)

	require.NotContains(t, string(content), "hunter2-hunter2", "base64-encoded, never the raw password")
	require.Contains(t, string(content), `"reg.example.com"`)
}
