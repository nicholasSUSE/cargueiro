package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const testCA = `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----
`

func newStore(t *testing.T) *Store {
	t.Helper()

	return Open(filepath.Join(t.TempDir(), "cfg", "login.json"))
}

func validLogin() Login {
	return Login{Host: "reg.example.com:5000", Username: "admin", Password: "s3cret: with spaces & ünïcode"}
}

func TestDefaultPath(t *testing.T) {
	t.Run("explicit directory wins", func(t *testing.T) {
		t.Setenv("CARGUEIRO_CONFIG_DIR", "/custom/dir")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")

		got, err := DefaultPath()

		require.NoError(t, err)
		require.Equal(t, "/custom/dir/login.json", got)
	})

	t.Run("XDG config home", func(t *testing.T) {
		t.Setenv("CARGUEIRO_CONFIG_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")

		got, err := DefaultPath()

		require.NoError(t, err)
		require.Equal(t, "/xdg/cargueiro/login.json", got)
	})

	t.Run("home directory fallback", func(t *testing.T) {
		t.Setenv("CARGUEIRO_CONFIG_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "/home/someone")

		got, err := DefaultPath()

		require.NoError(t, err)
		require.Equal(t, "/home/someone/.config/cargueiro/login.json", got)
	})
}

func TestSaveAndLoad(t *testing.T) {
	s := newStore(t)

	_, found, err := s.Load()
	require.NoError(t, err)
	require.False(t, found, "nothing is saved at first")
	require.NoFileExists(t, s.Path(), "loading creates nothing")

	login := validLogin()
	login.CAPEM = testCA
	require.NoError(t, s.Save(login))

	got, found, err := s.Load()
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, login, got, "everything comes back byte for byte, colons and all")
}

func TestSaveReplacesThePreviousLogin(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.Save(Login{Host: "a.example.com", Username: "alice", Password: "pa", PlainHTTP: true}))

	require.NoError(t, s.Save(Login{Host: "b.example.com", Username: "bob", Password: "pb"}))

	got, _, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, Login{Host: "b.example.com", Username: "bob", Password: "pb"}, got, "nothing of the first login is left")
}

func TestPlainHTTPAndCAAreSavedSeparately(t *testing.T) {
	s := newStore(t)

	require.NoError(t, s.Save(Login{Host: "h", Username: "u", Password: "p", PlainHTTP: true}))
	got, _, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, Login{Host: "h", Username: "u", Password: "p", PlainHTTP: true}, got)

	require.NoError(t, s.Save(Login{Host: "h", Username: "u", Password: "p", CAPEM: testCA}))
	got, _, err = s.Load()
	require.NoError(t, err)
	require.Equal(t, Login{Host: "h", Username: "u", Password: "p", CAPEM: testCA}, got)
}

func TestDelete(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.Save(validLogin()))

	existed, err := s.Delete()
	require.NoError(t, err)
	require.True(t, existed)
	require.NoFileExists(t, s.Path())
	_, found, err := s.Load()
	require.NoError(t, err)
	require.False(t, found)

	existed, err = s.Delete()
	require.NoError(t, err)
	require.False(t, existed, "deleting twice is not an error")
}

func TestPersistsAcrossOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login.json")
	require.NoError(t, Open(path).Save(validLogin()))

	got, found, err := Open(path).Load()

	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, validLogin(), got)
}

func TestFileAndDirectoryModes(t *testing.T) {
	s := newStore(t)

	require.NoError(t, s.Save(validLogin()))

	fileInfo, err := os.Stat(s.Path())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm(), "login file")
	dirInfo, err := os.Stat(filepath.Dir(s.Path()))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "its directory")
	entries, err := os.ReadDir(filepath.Dir(s.Path()))
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temporary file is left behind")

	t.Run("a loose file is tightened by the next save", func(t *testing.T) {
		require.NoError(t, os.Chmod(s.Path(), 0o644))

		require.NoError(t, s.Save(validLogin()))

		info, err := os.Stat(s.Path())
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})
}

func TestCheckPermissions(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.CheckPermissions(), "a missing file is fine")

	require.NoError(t, s.Save(validLogin()))
	require.NoError(t, s.CheckPermissions())

	for _, mode := range []os.FileMode{0o640, 0o604, 0o644, 0o666, 0o660} {
		require.NoError(t, os.Chmod(s.Path(), mode))
		require.ErrorIs(t, s.CheckPermissions(), ErrInsecurePermissions, "mode %04o", mode)
	}

	require.NoError(t, os.Chmod(s.Path(), 0o400))
	require.NoError(t, s.CheckPermissions(), "read-only for the owner is not a leak")
}

func TestSaveRejectsABadLogin(t *testing.T) {
	tests := []struct {
		name  string
		login Login
	}{
		{name: "empty host", login: Login{Username: "u", Password: "p"}},
		{name: "empty username", login: Login{Host: "h", Password: "p"}},
		{name: "empty password", login: Login{Host: "h", Username: "u"}},
		{name: "CA with plain HTTP", login: Login{Host: "h", Username: "u", Password: "p", PlainHTTP: true, CAPEM: testCA}},
		{name: "CA that is not PEM", login: Login{Host: "h", Username: "u", Password: "p", CAPEM: "not a certificate"}},
		{name: "PEM that is not a certificate", login: Login{Host: "h", Username: "u", Password: "p", CAPEM: "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStore(t)

			err := s.Save(tt.login)

			require.ErrorIs(t, err, ErrInvalidLogin)
			require.NoFileExists(t, s.Path(), "a rejected login leaves no file behind")
		})
	}
}

func TestACorruptFileIsReportedAndCanBeDeleted(t *testing.T) {
	s := newStore(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(s.Path()), 0o700))

	for name, content := range map[string]string{
		"not JSON":        "{ not json",
		"missing fields":  `{"host": "h"}`,
		"wrong data type": `{"host": 5}`,
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(s.Path(), []byte(content), 0o600))

			_, found, err := s.Load()

			require.Error(t, err)
			require.ErrorContains(t, err, "registry logout", "it says how to recover")
			require.False(t, found)
			existed, err := s.Delete()
			require.NoError(t, err)
			require.True(t, existed, "logout can remove it")
		})
	}
}

func TestMatches(t *testing.T) {
	login := Login{Host: "Registry.Example.COM:5000"}

	tests := []struct {
		host string
		want bool
	}{
		{host: "Registry.Example.COM:5000", want: true},
		{host: "registry.example.com:5000", want: true},
		{host: "REGISTRY.EXAMPLE.COM:5000", want: true},
		{host: "registry.example.com", want: false},
		{host: "registry.example.com:5001", want: false},
		{host: "other.example.com:5000", want: false},
		{host: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			require.Equal(t, tt.want, login.Matches(tt.host))
		})
	}

	require.False(t, Login{}.Matches(""), "an empty login matches nothing")
}
