//go:build e2e

package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// GitEnv returns the environment that makes git ignore the settings of whoever
// runs the tests, and gives it a name to commit as.
func GitEnv() []string {
	return []string{
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	}
}

// Git runs git in dir and returns what it printed. It fails the test if git fails.
func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()

	_, err := exec.LookPath("git")
	require.NoError(t, err, "git not found on PATH")

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), GitEnv()...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// ChartsCheckout makes a checkout like the staging branch of rancher/charts: a
// clone of a bare repository whose staging branch has one commit, which holds an
// empty index.yaml. It returns the checkout and the bare repository.
func ChartsCheckout(t *testing.T) (dir, remote string) {
	t.Helper()

	remote = filepath.Join(t.TempDir(), "charts.git")
	Git(t, t.TempDir(), "init", "--bare", "--initial-branch=staging", remote)

	seed := t.TempDir()
	Git(t, seed, "init", "--initial-branch=staging")
	require.NoError(t, os.WriteFile(filepath.Join(seed, "index.yaml"), nil, 0o600))
	Git(t, seed, "add", "index.yaml")
	Git(t, seed, "commit", "-m", "an empty index")
	Git(t, seed, "remote", "add", "origin", remote)
	Git(t, seed, "push", "origin", "staging")

	dir = filepath.Join(t.TempDir(), "charts")
	Git(t, t.TempDir(), "clone", "--branch", "staging", remote, dir)
	return dir, remote
}

// CommitAndPush commits everything in the checkout and pushes it to its remote.
func CommitAndPush(t *testing.T, dir, message string) {
	t.Helper()

	Git(t, dir, "add", "-A")
	Git(t, dir, "commit", "-m", message)
	Git(t, dir, "push", "origin", "HEAD")
}
