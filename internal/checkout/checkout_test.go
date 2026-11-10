package checkout

import (
	"context"
	"errors"
	"fmt"
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
			wantErr: "ask the remote origin (",
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

func TestVerifyFailsWhenThereIsNoRemote(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--initial-branch=staging")
	commit(t, dir, "first")

	err := Verify(context.Background(), dir, "staging", Git)

	require.ErrorContains(t, err, "has no git remote, so there is nothing to compare it with")
	require.ErrorContains(t, err, "clone rancher/charts instead of copying it, or add the remote with git remote add origin <url>")
}

func TestVerifyChecksTheRemoteBeforeAnythingElse(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--initial-branch=other")
	commit(t, dir, "first")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dirty"), []byte("x"), 0o600))

	err := Verify(context.Background(), dir, "staging", Git)

	require.ErrorContains(t, err, "has no git remote", "not the branch, and not the changes")
}

func TestVerifyFailsWhenTheBranchHasNoUpstream(t *testing.T) {
	remote := remoteWithStaging(t)

	t.Run("with one remote, the command is spelled out", func(t *testing.T) {
		dir := clone(t, remote)
		git(t, dir, "branch", "--unset-upstream")

		err := Verify(context.Background(), dir, "staging", Git)

		require.ErrorContains(t, err, "staging has no upstream")
		require.ErrorContains(t, err, "The remotes are: origin ("+remote+")")
		require.ErrorContains(t, err, "git fetch origin staging && git branch --set-upstream-to=origin/staging staging")
	})

	t.Run("with several, the remote is left to fill in", func(t *testing.T) {
		dir := clone(t, remote)
		git(t, dir, "branch", "--unset-upstream")
		git(t, dir, "remote", "add", "fork", "https://user:secret@github.com/someone/charts.git")

		err := Verify(context.Background(), dir, "staging", Git)

		require.ErrorContains(t, err, "origin ("+remote+")")
		require.ErrorContains(t, err, "fork (https://user:xxxxx@github.com/someone/charts.git)", "a password is never printed")
		require.NotContains(t, err.Error(), "secret")
		require.ErrorContains(t, err, "git fetch <remote> staging && git branch --set-upstream-to=<remote>/staging staging")
	})
}

func TestVerifyTellsWhatToDoAboutTheBranch(t *testing.T) {
	remote := remoteWithStaging(t)

	t.Run("another branch", func(t *testing.T) {
		dir := clone(t, remote)
		git(t, dir, "checkout", "-b", "feature")

		err := Verify(context.Background(), dir, "staging", Git)

		require.ErrorContains(t, err, "check it out with git checkout staging")
	})

	t.Run("no branch", func(t *testing.T) {
		dir := clone(t, remote)
		git(t, dir, "checkout", "--detach")

		err := Verify(context.Background(), dir, "staging", Git)

		require.ErrorContains(t, err, "check it out with git checkout staging")
		require.ErrorContains(t, err, "in GitHub Actions, give actions/checkout the ref staging")
	})
}

func TestVerifyPicksTheBranchAndNothingThatEndsLikeIt(t *testing.T) {
	remote := remoteWithStaging(t)
	dir := clone(t, remote)
	// A ref whose name ends with the one asked for: ls-remote tail-matches.
	seed := clone(t, remote)
	commit(t, seed, "other")
	git(t, seed, "push", "origin", "HEAD:refs/heads/a/refs/heads/staging") // sorts before the real one

	require.NoError(t, Verify(context.Background(), dir, "staging", Git))
}

// scripted is a Runner that answers from a table, by the first words of the
// command, and records what it was asked.
type scripted struct {
	answers map[string]answer
	asked   []string
}

type answer struct {
	stdout string
	stderr string
	err    error
}

func (s *scripted) run(_ context.Context, _ string, args ...string) (string, error) {
	command := strings.Join(args, " ")
	s.asked = append(s.asked, command)

	// The longest prefix is the one that was meant.
	var best string
	for prefix := range s.answers {
		if strings.HasPrefix(command, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" {
		return "", fmt.Errorf("the test did not expect: git %s", command)
	}

	a := s.answers[best]
	if a.err != nil || a.stderr != "" {
		err := a.err
		if err == nil {
			err = errors.New("exit status 128")
		}
		return a.stdout, &Error{Args: args, Stderr: a.stderr, Err: err}
	}
	return a.stdout, nil
}

// healthy answers like a checkout that is fine, for the folder dir.
func healthy(dir string) map[string]answer {
	return map[string]answer{
		"--version":                                       {stdout: "git version 2.52.0\n"},
		"rev-parse --show-toplevel":                       {stdout: dir + "\n"},
		"remote get-url origin":                           {stdout: "https://github.com/rancher/charts.git\n"},
		"remote":                                          {stdout: "origin\n"},
		"rev-parse --abbrev-ref HEAD":                     {stdout: "staging\n"},
		"--no-optional-locks status --porcelain":          {},
		"config --get branch.staging.remote":              {stdout: "origin\n"},
		"config --get branch.staging.merge":               {stdout: "refs/heads/staging\n"},
		"ls-remote --exit-code origin refs/heads/staging": {stdout: "aaaa\trefs/heads/staging\n"},
		"rev-parse HEAD":                                  {stdout: "aaaa\n"},
	}
}

func TestVerifyExplainsWhyTheRemoteCannotBeAsked(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name   string
		stderr string
		want   []string
	}{
		{
			name:   "credentials that are not there",
			stderr: "fatal: could not read Username for 'https://github.com': terminal prompts disabled",
			want: []string{
				"ask the remote origin (https://github.com/rancher/charts.git) for its latest commit",
				"terminal prompts disabled",
				"needs credentials that this machine does not have",
				"persist-credentials",
			},
		},
		{
			name:   "an ssh key that is not accepted",
			stderr: "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.",
			want:   []string{"needs credentials that this machine does not have"},
		},
		{
			name:   "a repository GitHub hides from those without access",
			stderr: "remote: Repository not found.\nfatal: repository 'https://github.com/rancher/charts.git/' not found",
			want:   []string{"needs credentials that this machine does not have"},
		},
		{
			name:   "a host name that only some machines know",
			stderr: "ssh: Could not resolve hostname github-work: Name or service not known\nfatal: Could not read from remote repository.",
			want: []string{
				"Could not resolve hostname github-work",
				"cannot be reached from this machine",
				"a host alias from ~/.ssh/config only exists on the machine that has it",
			},
		},
		{
			name:   "no network",
			stderr: "fatal: unable to access 'https://github.com/rancher/charts.git/': Could not resolve host: github.com",
			want:   []string{"cannot be reached from this machine"},
		},
		{
			name:   "something else",
			stderr: "fatal: the remote hung up unexpectedly",
			want:   []string{"the remote hung up unexpectedly"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := healthy(dir)
			answers["ls-remote --exit-code origin refs/heads/staging"] = answer{stderr: tt.stderr}
			s := &scripted{answers: answers}

			err := Verify(context.Background(), dir, "staging", s.run)

			for _, want := range tt.want {
				require.ErrorContains(t, err, want)
			}
		})
	}
}

func TestVerifyExplainsADubiousOwner(t *testing.T) {
	dir := t.TempDir()
	answers := healthy(dir)
	answers["rev-parse --show-toplevel"] = answer{stderr: "fatal: detected dubious ownership in repository at '" + dir + "'"}
	s := &scripted{answers: answers}

	err := Verify(context.Background(), dir, "staging", s.run)

	require.ErrorContains(t, err, "is not a git checkout")
	require.ErrorContains(t, err, "dubious ownership")
	require.ErrorContains(t, err, "git config --global --add safe.directory "+dir)
}

func TestVerifyWhenGitIsNotInstalled(t *testing.T) {
	s := &scripted{answers: map[string]answer{"--version": {err: exec.ErrNotFound}}}

	err := Verify(context.Background(), t.TempDir(), "staging", s.run)

	require.ErrorContains(t, err, "cannot run git")
	require.ErrorIs(t, err, exec.ErrNotFound)
}

func TestVerifyGivesUpOnARemoteThatDoesNotAnswer(t *testing.T) {
	dir := t.TempDir()
	answers := healthy(dir)
	answers["ls-remote --exit-code origin refs/heads/staging"] = answer{err: context.DeadlineExceeded}
	s := &scripted{answers: answers}

	err := Verify(context.Background(), dir, "staging", s.run)

	require.ErrorContains(t, err, "did not answer in")
}

func TestVerifyDoesNotPrintAPasswordInTheRemote(t *testing.T) {
	dir := t.TempDir()
	answers := healthy(dir)
	answers["remote get-url origin"] = answer{stdout: "https://x-access-token:ghs_SECRET@github.com/rancher/charts.git\n"}
	answers["ls-remote --exit-code origin refs/heads/staging"] = answer{stderr: "fatal: unable to access 'https://x-access-token:ghs_SECRET@github.com/rancher/charts.git/': The requested URL returned error: 403"}
	s := &scripted{answers: answers}

	err := Verify(context.Background(), dir, "staging", s.run)

	require.Error(t, err)
	require.NotContains(t, err.Error(), "ghs_SECRET")
	require.Contains(t, err.Error(), "x-access-token:xxxxx@github.com")
}
