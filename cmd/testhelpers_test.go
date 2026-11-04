package cmd

import (
	"path/filepath"
	"testing"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
)

// newTestStore returns a login store in a temporary directory.
func newTestStore(t *testing.T) *auth.Store {
	t.Helper()

	return auth.Open(filepath.Join(t.TempDir(), "cfg", "login.json"))
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
