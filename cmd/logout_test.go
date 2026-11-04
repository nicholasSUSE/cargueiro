package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

func TestLogout(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	for _, host := range []string{"a.example.com", "b.example.com"} {
		_, err := store.Put(ctx, host, auth.Credential{Username: "u", Password: "p"})
		require.NoError(t, err)
	}

	res := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout", "a.example.com")

	require.NoError(t, res.err)
	require.Equal(t, "Removed credentials for a.example.com\n", res.stdout)
	require.Empty(t, res.calls, "logout never touches the network")
	_, found := saved(t, store, "a.example.com")
	require.False(t, found)
	_, found = saved(t, store, "b.example.com")
	require.True(t, found, "other registries keep their credentials")

	t.Run("logging out twice is not an error", func(t *testing.T) {
		again := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout", "a.example.com")

		require.NoError(t, again.err)
		require.Equal(t, "No credentials stored for a.example.com\n", again.stdout)
	})
}

func TestLogoutWithNoCredentialsFile(t *testing.T) {
	store := newTestStore(t)

	res := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout", "reg.example.com")

	require.NoError(t, res.err)
	require.Equal(t, "No credentials stored for reg.example.com\n", res.stdout)
	require.NoFileExists(t, store.Path(), "logging out must not create the file")
}

func TestLogoutHostForms(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	_, err := store.Put(ctx, "reg.example.com:5000", auth.Credential{Username: "u", Password: "p"})
	require.NoError(t, err)

	res := runRegistry(t, store, &fakePrompter{}, nil, "", true, nil, "logout", "https://REG.example.com:5000/")

	require.NoError(t, res.err)
	require.Equal(t, "Removed credentials for REG.example.com:5000\n", res.stdout)
}

func TestLogoutRefusesAnAddressWithAPath(t *testing.T) {
	res := runRegistry(t, newTestStore(t), &fakePrompter{}, nil, "", true, nil, "logout", "reg.example.com/charts")

	require.ErrorIs(t, res.err, oci.ErrInvalidRegistry)
}

func TestLogoutNeedsExactlyOneHost(t *testing.T) {
	for _, args := range [][]string{{"logout"}, {"logout", "a", "b"}} {
		res := runRegistry(t, newTestStore(t), &fakePrompter{}, nil, "", true, nil, args...)

		require.Error(t, res.err, "%v", args)
	}
}
