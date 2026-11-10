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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), redactText(e.Stderr))
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
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("git is not installed, or is not on the PATH: %w", err)
		}
		return stdout.String(), &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.String(), nil
}

// askTimeout is how long the remote has to answer.
const askTimeout = time.Minute

// Verify fails unless dir is the root of a git checkout that has a remote, is on
// branch, has nothing uncommitted, follows branch of a remote, and is at the
// latest commit of that branch on the remote. The remote is asked for it, so a
// checkout that has not fetched in a while is not trusted. The error says which
// of these is not so, and what to do about it.
func Verify(ctx context.Context, dir, branch string, run Runner) error {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder", dir)
	}
	if _, err := run(ctx, dir, "--version"); err != nil {
		return fmt.Errorf("cannot run git: %w", err)
	}

	if err := verifyRoot(ctx, dir, run); err != nil {
		return err
	}
	remotes, err := verifyRemotes(ctx, dir, run)
	if err != nil {
		return err
	}
	if err := verifyBranch(ctx, dir, branch, run); err != nil {
		return err
	}
	if err := verifyClean(ctx, dir, run); err != nil {
		return err
	}
	remote, err := verifyUpstream(ctx, dir, branch, remotes, run)
	if err != nil {
		return err
	}
	return verifyLatest(ctx, dir, branch, remote, run)
}

// verifyRoot checks that dir is the top folder of a checkout.
func verifyRoot(ctx context.Context, dir string, run Runner) error {
	top, err := run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("%s is not a git checkout: %w%s", dir, err, hintFor(err, dir))
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

// verifyRemotes checks that the checkout has a remote at all, and returns their
// names.
func verifyRemotes(ctx context.Context, dir string, run Runner) ([]string, error) {
	listed, err := run(ctx, dir, "remote")
	if err != nil {
		return nil, fmt.Errorf("list the remotes of %s: %w", dir, err)
	}
	remotes := strings.Fields(listed)
	if len(remotes) == 0 {
		return nil, fmt.Errorf("%s has no git remote, so there is nothing to compare it with: clone rancher/charts instead of copying it, or add the remote with git remote add origin <url>", dir)
	}
	return remotes, nil
}

// describeRemotes lists the remotes with their addresses, as "origin (url), up
// (url)". An address that cannot be read is left out.
func describeRemotes(ctx context.Context, dir string, remotes []string, run Runner) string {
	described := make([]string, 0, len(remotes))
	for _, name := range remotes {
		address, err := run(ctx, dir, "remote", "get-url", name)
		if err != nil {
			described = append(described, name)
			continue
		}
		described = append(described, fmt.Sprintf("%s (%s)", name, redactURL(strings.TrimSpace(address))))
	}
	return strings.Join(described, ", ")
}

// verifyBranch checks that the checkout is on branch.
func verifyBranch(ctx context.Context, dir, branch string, run Runner) error {
	current, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("find the branch of %s: %w", dir, err)
	}
	switch current = strings.TrimSpace(current); current {
	case "HEAD":
		return fmt.Errorf("%s is not on a branch, and it must be on %s: check it out with git checkout %s (in GitHub Actions, give actions/checkout the ref %s)", dir, branch, branch, branch)
	case branch:
		return nil
	}
	return fmt.Errorf("%s is on the branch %s, and it must be on %s: check it out with git checkout %s", dir, current, branch, branch)
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
func verifyUpstream(ctx context.Context, dir, branch string, remotes []string, run Runner) (string, error) {
	remote, err := config(ctx, dir, run, "branch."+branch+".remote")
	if err != nil {
		return "", err
	}
	merge, err := config(ctx, dir, run, "branch."+branch+".merge")
	if err != nil {
		return "", err
	}
	if remote == "" || merge == "" {
		name := "<remote>"
		if len(remotes) == 1 {
			name = remotes[0]
		}
		return "", fmt.Errorf("%s has no upstream. The remotes are: %s. Set the upstream with git fetch %s %s && git branch --set-upstream-to=%s/%s %s",
			branch, describeRemotes(ctx, dir, remotes, run), name, branch, name, branch, branch)
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
	ref := "refs/heads/" + branch
	asked, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()

	listed, err := run(asked, dir, "ls-remote", "--exit-code", remote, ref)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 2 {
		return fmt.Errorf("the remote %s has no branch %s", remote, branch)
	}
	if err != nil {
		return fmt.Errorf("ask the remote %s (%s) for its latest commit: %s", remote, remoteAddress(ctx, dir, remote, run), explainAsk(err))
	}
	latest := commitOf(listed, ref)
	if latest == "" {
		return fmt.Errorf("ask the remote %s for its latest commit: it has no %s in what it printed: %q", remote, ref, listed)
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

// commitOf returns the commit that the output of git ls-remote gives for exactly
// ref. It is not enough to look at the first line: a pattern is matched against
// the end of the names, so refs/heads/x/refs/heads/staging matches too.
func commitOf(listed, ref string) string {
	for _, line := range strings.Split(listed, "\n") {
		commit, name, found := strings.Cut(strings.TrimSpace(line), "\t")
		if found && name == ref {
			return commit
		}
	}
	return ""
}

// remoteAddress returns the address of the remote, without a password.
func remoteAddress(ctx context.Context, dir, remote string, run Runner) string {
	address, err := run(ctx, dir, "remote", "get-url", remote)
	if err != nil {
		return "address unknown"
	}
	return redactURL(strings.TrimSpace(address))
}

// explainAsk says why the remote could not be asked, and what to do about it.
func explainAsk(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("it did not answer in %s: check the network and the address of the remote", askTimeout)
	}
	return err.Error() + hintFor(err, "")
}

// hintFor returns a line that says what to do about a git error that has a
// known cause, starting with a new line, or "" when it has none. dir is the
// checkout the error came from, when it is known.
func hintFor(err error, dir string) string {
	var failed *Error
	if !errors.As(err, &failed) {
		return ""
	}
	stderr := strings.ToLower(failed.Stderr)
	has := func(parts ...string) bool {
		for _, part := range parts {
			if strings.Contains(stderr, part) {
				return true
			}
		}
		return false
	}

	switch {
	case has("dubious ownership"):
		return "\ngit does not trust the owner of the folder: tell it to with git config --global --add safe.directory " + dir
	case has("terminal prompts disabled", "authentication failed", "could not read username", "permission denied (publickey)",
		"repository not found", "error: 401", "error: 403", "invalid username or password"):
		return "\nthe remote needs credentials that this machine does not have: in GitHub Actions leave persist-credentials on for actions/checkout, or give it a token; elsewhere sign in with a credential helper or an ssh key"
	case has("could not resolve host", "name or service not known", "network is unreachable", "connection timed out", "connection refused"):
		return "\nthe remote cannot be reached from this machine: check the network and the address; a host alias from ~/.ssh/config only exists on the machine that has it, so use another remote or an https address"
	}
	return ""
}

// passwordInText matches the password in an address inside a text.
var passwordInText = regexp.MustCompile(`(://[^/\s:@'"]+):[^@/\s'"]+@`)

// redactText hides the passwords of the addresses in text.
func redactText(text string) string {
	return passwordInText.ReplaceAllString(text, "${1}:xxxxx@")
}

// redactURL hides what in the address of a remote could be a secret: a password,
// or the user of an http address, which is often a token.
func redactURL(address string) string {
	u, err := url.Parse(address)
	if err != nil || u.User == nil || u.Scheme == "" {
		return redactText(address)
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		u.User = url.UserPassword(u.User.Username(), "xxxxx")
	} else if u.Scheme == "http" || u.Scheme == "https" {
		u.User = url.User("xxxxx")
	}
	return u.String()
}
