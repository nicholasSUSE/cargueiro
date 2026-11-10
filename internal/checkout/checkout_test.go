package checkout

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// git runs git in dir, as a user with a name, and returns what it printed. It
// fails the test if git fails.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// commit adds a file to the checkout in dir and commits it.
func commit(t *testing.T, dir, file string) string {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(file), 0o600))
	git(t, dir, "add", file)
	git(t, dir, "commit", "-m", file)
	return git(t, dir, "rev-parse", "HEAD")
}

// remoteWithStaging makes a bare repository with a staging branch that has a
// commit, and returns its path.
func remoteWithStaging(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is not installed")
	}
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, t.TempDir(), "init", "--bare", "--initial-branch=staging", remote)

	seed := t.TempDir()
	git(t, seed, "init", "--initial-branch=staging")
	commit(t, seed, "first")
	git(t, seed, "remote", "add", "origin", remote)
	git(t, seed, "push", "origin", "staging")
	return remote
}

// clone clones the remote into a new folder, on staging.
func clone(t *testing.T, remote string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "charts")
	git(t, t.TempDir(), "clone", "--branch", "staging", remote, dir)
	return dir
}

func TestVerify(t *testing.T) {
	remote := remoteWithStaging(t)
	dir := clone(t, remote)

	require.NoError(t, Verify(context.Background(), dir, "staging", Git))
}

func TestVerifyAnotherBranch(t *testing.T) {
	remote := remoteWithStaging(t)
	dir := clone(t, remote)
	git(t, dir, "checkout", "-b", "other")
	git(t, dir, "push", "--set-upstream", "origin", "other")

	require.NoError(t, Verify(context.Background(), dir, "other", Git))
	require.ErrorContains(t, Verify(context.Background(), dir, "staging", Git), "is on the branch other, and it must be on staging")
}

func TestVerifyFails(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr string
	}{
		{
			name: "a folder that is not a checkout",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: "is not a git checkout",
		},
		{
			name: "a folder inside a checkout",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				inside := filepath.Join(dir, "entries")
				require.NoError(t, os.Mkdir(inside, 0o750))
				return inside
			},
			wantErr: "is not the root of its git checkout",
		},
		{
			name: "a folder that is not there",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing")
			},
			wantErr: "is not a folder",
		},
		{
			name: "another branch",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				git(t, dir, "checkout", "-b", "feature")
				return dir
			},
			wantErr: "is on the branch feature, and it must be on staging",
		},
		{
			name: "no branch at all",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				git(t, dir, "checkout", "--detach")
				return dir
			},
			wantErr: "is not on a branch, and it must be on staging",
		},
		{
			name: "a tracked file that changed",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "first"), []byte("changed"), 0o600))
				return dir
			},
			wantErr: "has changes that are not committed",
		},
		{
			name: "a file that is not tracked",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "new"), []byte("x"), 0o600))
				return dir
			},
			wantErr: "?? new",
		},
		{
			name: "a branch with no upstream",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				git(t, dir, "branch", "--unset-upstream")
				return dir
			},
			wantErr: "staging has no upstream",
		},
		{
			name: "an upstream that is another branch",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				git(t, dir, "push", "origin", "staging:main")
				git(t, dir, "branch", "--set-upstream-to=origin/main")
				return dir
			},
			wantErr: "staging follows main, and it must follow staging",
		},
		{
			name: "the upstream is a local branch",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				git(t, dir, "branch", "local", "staging")
				git(t, dir, "branch", "--set-upstream-to=local")
				return dir
			},
			wantErr: "follows a local branch",
		},
		{
			name: "the branch is gone from the remote",
			setup: func(t *testing.T) string {
				remote := remoteWithStaging(t)
				dir := clone(t, remote)
				git(t, dir, "push", "origin", "staging:other")
				git(t, remote, "symbolic-ref", "HEAD", "refs/heads/other")
				git(t, remote, "branch", "-D", "staging")
				return dir
			},
			wantErr: "has no branch staging",
		},
		{
			name: "the remote cannot be reached",
			setup: func(t *testing.T) string {
				dir := clone(t, remoteWithStaging(t))
				git(t, dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "nowhere.git"))
				return dir
			},
			wantErr: "ask the remote for its latest commit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)

			err := Verify(context.Background(), dir, "staging", Git)

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestVerifyFailsWhenTheCheckoutIsBehind(t *testing.T) {
	remote := remoteWithStaging(t)
	behind := clone(t, remote)
	ahead := clone(t, remote)
	latest := commit(t, ahead, "second")
	git(t, ahead, "push", "origin", "staging")
	old := git(t, behind, "rev-parse", "HEAD")

	err := Verify(context.Background(), behind, "staging", Git)

	require.ErrorContains(t, err, "is at "+old+" and staging is at "+latest+" upstream: update it")
}

func TestVerifyFailsWhenTheCheckoutHasACommitThatIsNotPushed(t *testing.T) {
	remote := remoteWithStaging(t)
	dir := clone(t, remote)
	pushed := git(t, dir, "rev-parse", "HEAD")
	local := commit(t, dir, "second")

	err := Verify(context.Background(), dir, "staging", Git)

	require.ErrorContains(t, err, "is at "+local+" and staging is at "+pushed+" upstream: update it")
}

func TestVerifyLooksAtTheRemoteAndNotAtWhatWasFetched(t *testing.T) {
	remote := remoteWithStaging(t)
	dir := clone(t, remote)
	other := clone(t, remote)
	commit(t, other, "second")
	git(t, other, "push", "origin", "staging")
	// dir has not fetched, so its own origin/staging is still the old commit.
	require.Equal(t, git(t, dir, "rev-parse", "HEAD"), git(t, dir, "rev-parse", "origin/staging"))

	err := Verify(context.Background(), dir, "staging", Git)

	require.ErrorContains(t, err, "upstream: update it")
}

func TestVerifyChangesNothing(t *testing.T) {
	remote := remoteWithStaging(t)
	dir := clone(t, remote)
	before := git(t, dir, "status", "--porcelain=v2", "--branch")
	index := fileInfo(t, filepath.Join(dir, ".git", "index"))

	require.NoError(t, Verify(context.Background(), dir, "staging", Git))
	indexAfter := fileInfo(t, filepath.Join(dir, ".git", "index"))

	require.Equal(t, index, indexAfter, "the index is not rewritten")
	require.Equal(t, before, git(t, dir, "status", "--porcelain=v2", "--branch"))
}

func fileInfo(t *testing.T, path string) string {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.ModTime().String() + " " + info.Mode().String()
}
