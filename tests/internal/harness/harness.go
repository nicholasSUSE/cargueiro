//go:build e2e

// Package harness holds the helpers shared by the e2e test packages: building
// and running the cargueiro binary, running the pinned helm CLI, and locating
// the test data.
package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/chart/archivetest"
)

// binEnv names the environment variable holding the path of the cargueiro
// binary under test. When unset, Main builds one from the working tree.
const binEnv = "CARGUEIRO_BIN"

const commandTimeout = 2 * time.Minute

// Result is the outcome of running a command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Main is meant for TestMain: it builds the binary under test (unless
// CARGUEIRO_BIN points at one), runs the tests and cleans up.
func Main(m *testing.M) int {
	if os.Getenv(binEnv) == "" {
		dir, err := os.MkdirTemp("", "cargueiro-e2e-")
		if err != nil {
			fmt.Fprintln(os.Stderr, "create build dir:", err)
			return 1
		}
		defer func() { _ = os.RemoveAll(dir) }()

		bin := filepath.Join(dir, "cargueiro")
		if err := build(bin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := os.Setenv(binEnv, bin); err != nil {
			fmt.Fprintln(os.Stderr, "set", binEnv+":", err)
			return 1
		}
	}

	return m.Run()
}

func build(out string) error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build cargueiro: %w\n%s", err, output)
	}
	return nil
}

// RepoRoot returns the module root.
func RepoRoot(t *testing.T) string {
	t.Helper()

	root, err := findRepoRoot()
	require.NoError(t, err)
	return root
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above the working directory")
		}
		dir = parent
	}
}

// Options tune how a command is run.
type Options struct {
	// Dir is the working directory.
	Dir string
	// Env adds KEY=value entries to a scrubbed copy of the process environment.
	Env []string
	// Stdin is fed to the command's standard input.
	Stdin string
}

// Cargueiro runs the binary under test in dir.
func Cargueiro(t *testing.T, dir string, args ...string) Result {
	t.Helper()

	return CargueiroWith(t, Options{Dir: dir}, args...)
}

// CargueiroWith runs the binary under test with opts.
func CargueiroWith(t *testing.T, opts Options, args ...string) Result {
	t.Helper()

	bin := os.Getenv(binEnv)
	require.NotEmpty(t, bin, "%s is unset: run the tests through a TestMain calling harness.Main", binEnv)

	return run(t, opts, bin, args...)
}

// Helm runs the helm CLI found on PATH in dir.
func Helm(t *testing.T, dir string, args ...string) Result {
	t.Helper()

	return HelmWith(t, Options{Dir: dir}, args...)
}

// HelmWith runs the helm CLI found on PATH with opts.
func HelmWith(t *testing.T, opts Options, args ...string) Result {
	t.Helper()

	_, err := exec.LookPath("helm")
	require.NoError(t, err, "helm not found on PATH: run `mise install` and activate mise (see docs/DEVELOPING.md)")

	return run(t, opts, "helm", args...)
}

// scrubbedEnv returns the process environment without variables that would
// leak credentials into a command under test.
func scrubbedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CARGUEIRO_USERNAME=") || strings.HasPrefix(kv, "CARGUEIRO_PASSWORD=") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func hasEnv(env []string, key string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			return true
		}
	}
	return false
}

func run(t *testing.T, opts Options, name string, args ...string) Result {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = opts.Dir
	env := append(scrubbedEnv(), opts.Env...)
	if !hasEnv(opts.Env, "CARGUEIRO_CONFIG_DIR") {
		// Never read or write the developer's real saved credentials.
		env = append(env, "CARGUEIRO_CONFIG_DIR="+t.TempDir())
	}
	cmd.Env = env
	cmd.Stdin = strings.NewReader(opts.Stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	res := Result{}
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		require.ErrorAsf(t, err, &exitErr, "run %s %v", name, args)
		res.ExitCode = exitErr.ExitCode()
	}
	res.Stdout, res.Stderr = stdout.String(), stderr.String()
	return res
}

// FleetSource returns the directory of a fleet chart's source.
func FleetSource(t *testing.T, chart string) string {
	t.Helper()

	return filepath.Join(RepoRoot(t), "tests", "data", "charts", "fleet", "src", chart)
}

// FleetPublished returns the path of a fleet chart's published archive.
func FleetPublished(t *testing.T, chart, version string) string {
	t.Helper()

	return filepath.Join(RepoRoot(t), "tests", "data", "charts", "fleet", "published", chart+"-"+version+".tgz")
}

// Manifest returns the content manifest of an archive.
func Manifest(t *testing.T, archive string) string {
	t.Helper()

	m, err := archivetest.ManifestFile(archive)
	require.NoError(t, err)
	return m
}

var helmSDKVersion = regexp.MustCompile(`(?m)^\s*helm\.sh/helm/v4\s+(v\S+)`)

// RequireHelmMatchesSDK fails unless the helm CLI on PATH has the same version
// as the helm.sh/helm/v4 module in go.mod, so that any difference between the
// two comes from our code and not from a version gap.
func RequireHelmMatchesSDK(t *testing.T) {
	t.Helper()

	gomod, err := os.ReadFile(filepath.Join(RepoRoot(t), "go.mod"))
	require.NoError(t, err)
	match := helmSDKVersion.FindSubmatch(gomod)
	require.NotNil(t, match, "helm.sh/helm/v4 not found in go.mod")

	res := Helm(t, t.TempDir(), "version", "--template", "{{.Version}}")
	require.Zero(t, res.ExitCode, res.Stderr)

	require.Equal(t, string(match[1]), strings.TrimSpace(res.Stdout),
		"helm CLI on PATH differs from the helm SDK in go.mod; fix mise.toml, go.mod or PATH (see docs/DEVELOPING.md)")
}
