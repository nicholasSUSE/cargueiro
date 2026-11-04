package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
)

// newTestStore returns a credential store in a temporary directory, with no
// native helper so tests never touch the machine's keychain.
func newTestStore(t *testing.T) *auth.Store {
	t.Helper()

	store, err := auth.Open(filepath.Join(t.TempDir(), "cfg", "auth.json"), auth.Options{})
	require.NoError(t, err)
	return store
}

// openerFor returns an openStore function that always hands out store.
func openerFor(store *auth.Store) func() (*auth.Store, error) {
	return func() (*auth.Store, error) { return store, nil }
}

// fakePrompter answers prompts from canned values and records what was asked.
type fakePrompter struct {
	interactive bool
	line        string
	secret      string
	asked       []string
}

func (f *fakePrompter) Interactive() bool { return f.interactive }

func (f *fakePrompter) Line(prompt string) (string, error) {
	f.asked = append(f.asked, prompt)
	return f.line, nil
}

func (f *fakePrompter) Secret(prompt string) (string, error) {
	f.asked = append(f.asked, prompt)
	return f.secret, nil
}
