// Package checkout checks that a folder is the checkout it is expected to be
// before something is written into it: the root of a git checkout, on the right
// branch, with nothing uncommitted, at the latest commit of that branch on its
// remote. It only reads: it runs the git binary for reads, never fetches and
// never changes the checkout, and it needs no credentials of its own.
package checkout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner runs git with args in dir and returns what it printed on its standard
// output.
type Runner func(ctx context.Context, dir string, args ...string) (string, error)

// Error is a git command that failed.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), e.Stderr)
}

func (e *Error) Unwrap() error { return e.Err }

// Git is the Runner that runs the git binary. It never asks for a password, and
// does not take the optional locks that would rewrite the index.
func Git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.String(), nil
}

// Verify fails unless dir is the root of a git checkout that is on branch, has
// nothing uncommitted, follows branch of a remote, and is at the latest commit
// of that branch on the remote. The remote is asked for it, so a checkout that
// has not fetched in a while is not trusted. The error says which of these is
// not so.
func Verify(ctx context.Context, dir, branch string, run Runner) error {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder", dir)
	}

	if err := verifyRoot(ctx, dir, run); err != nil {
		return err
	}
	if err := verifyBranch(ctx, dir, branch, run); err != nil {
		return err
	}
	if err := verifyClean(ctx, dir, run); err != nil {
		return err
	}
	remote, err := verifyUpstream(ctx, dir, branch, run)
	if err != nil {
		return err
	}
	return verifyLatest(ctx, dir, branch, remote, run)
}

// verifyRoot checks that dir is the top folder of a checkout.
func verifyRoot(ctx context.Context, dir string, run Runner) error {
	top, err := run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("%s is not a git checkout: %w", dir, err)
	}
	top = strings.TrimSpace(top)

	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", dir, err)
	}
	got, err := filepath.EvalSymlinks(top)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", top, err)
	}
	if want != got {
		return fmt.Errorf("%s is not the root of its git checkout, which is %s", dir, top)
	}
	return nil
}

// verifyBranch checks that the checkout is on branch.
func verifyBranch(ctx context.Context, dir, branch string, run Runner) error {
	current, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("find the branch of %s: %w", dir, err)
	}
	switch current = strings.TrimSpace(current); current {
	case "HEAD":
		return fmt.Errorf("%s is not on a branch, and it must be on %s", dir, branch)
	case branch:
		return nil
	}
	return fmt.Errorf("%s is on the branch %s, and it must be on %s", dir, current, branch)
}

// maxChanges is how many changed files an error lists.
const maxChanges = 10

// verifyClean checks that nothing is changed, added or left out of git.
func verifyClean(ctx context.Context, dir string, run Runner) error {
	status, err := run(ctx, dir, "--no-optional-locks", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("read the status of %s: %w", dir, err)
	}

	changes := strings.Split(strings.TrimRight(status, "\n"), "\n")
	if len(changes) == 1 && changes[0] == "" {
		return nil
	}
	listed := changes
	if len(listed) > maxChanges {
		listed = append(listed[:maxChanges:maxChanges], fmt.Sprintf("and %d more", len(changes)-maxChanges))
	}
	return fmt.Errorf("%s has changes that are not committed:\n%s", dir, strings.Join(listed, "\n"))
}

// verifyUpstream checks that branch follows the same branch of a remote, and
// returns the name of the remote.
func verifyUpstream(ctx context.Context, dir, branch string, run Runner) (string, error) {
	remote, err := config(ctx, dir, run, "branch."+branch+".remote")
	if err != nil {
		return "", err
	}
	merge, err := config(ctx, dir, run, "branch."+branch+".merge")
	if err != nil {
		return "", err
	}
	if remote == "" || merge == "" {
		return "", fmt.Errorf("%s has no upstream: set one with git branch --set-upstream-to", branch)
	}
	if remote == "." {
		return "", fmt.Errorf("%s follows a local branch, and it must follow a branch of a remote", branch)
	}
	if followed := strings.TrimPrefix(merge, "refs/heads/"); followed != branch {
		return "", fmt.Errorf("%s follows %s, and it must follow %s", branch, followed, branch)
	}
	return remote, nil
}

// config returns the value of a git setting, or "" when it is not set.
func config(ctx context.Context, dir string, run Runner, key string) (string, error) {
	value, err := run(ctx, dir, "config", "--get", key)
	var failed *Error
	var exit *exec.ExitError
	if errors.As(err, &failed) && errors.As(failed.Err, &exit) && exit.ExitCode() == 1 {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	return strings.TrimSpace(value), nil
}

// verifyLatest checks that HEAD is the latest commit of branch on the remote.
func verifyLatest(ctx context.Context, dir, branch, remote string, run Runner) error {
	listed, err := run(ctx, dir, "ls-remote", "--exit-code", remote, "refs/heads/"+branch)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 2 {
		return fmt.Errorf("the remote %s has no branch %s", remote, branch)
	}
	if err != nil {
		return fmt.Errorf("ask the remote for its latest commit: %w", err)
	}
	latest, _, _ := strings.Cut(strings.TrimSpace(listed), "\t")
	if latest == "" {
		return fmt.Errorf("ask the remote for its latest commit: git ls-remote printed %q", listed)
	}

	head, err := run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read the commit of %s: %w", dir, err)
	}
	if head = strings.TrimSpace(head); head != latest {
		return fmt.Errorf("the checkout is at %s and %s is at %s upstream: update it", head, branch, latest)
	}
	return nil
}
