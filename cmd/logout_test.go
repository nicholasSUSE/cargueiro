package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
)

func TestLogoutForgetsTheSavedLogin(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(auth.Login{Host: "a.example.com", Username: "u", Password: "p", PlainHTTP: true}))

	res := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout")

	require.NoError(t, res.err)
	require.Equal(t, "Removed the saved login for a.example.com\n", res.stdout)
	require.Empty(t, res.calls, "logout never touches the network")
	require.NoFileExists(t, store.Path(), "credentials and transport settings go together")

	t.Run("logging out twice is not an error", func(t *testing.T) {
		again := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout")

		require.NoError(t, again.err)
		require.Equal(t, "No saved login\n", again.stdout)
	})
}

func TestLogoutWithNothingSavedCreatesNothing(t *testing.T) {
	store := newTestStore(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout")

	require.NoError(t, res.err)
	require.Equal(t, "No saved login\n", res.stdout)
	require.NoFileExists(t, store.Path())
}

func TestLogoutRemovesACorruptLoginFile(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(store.Path()), 0o700))
	require.NoError(t, os.WriteFile(store.Path(), []byte("{ not json"), 0o600))

	res := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout")

	require.NoError(t, res.err)
	require.Equal(t, "Removed the saved login\n", res.stdout)
	require.NoFileExists(t, store.Path(), "it is how the user recovers")
}

func TestLogoutTakesNoArguments(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Save(auth.Login{Host: "a.example.com", Username: "u", Password: "p"}))

	res := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout", "a.example.com")

	require.ErrorContains(t, res.err, "unknown command")
	_, found := saved(t, store)
	require.True(t, found, "a mistaken argument must not log out")
}
