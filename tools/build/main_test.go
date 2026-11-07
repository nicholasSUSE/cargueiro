package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargets(t *testing.T) {
	want := []target{
		{os: "linux", arch: "amd64"},
		{os: "darwin", arch: "amd64"},
		{os: "darwin", arch: "arm64"},
		{os: "freebsd", arch: "amd64"},
	}
	require.Equal(t, want, targets())
}

func TestCommand(t *testing.T) {
	cfg := config{
		version: "v0.1.0",
		commit:  "abc1234",
		outDir:  "out",
	}

	cmd := cfg.command(context.Background(), target{os: "darwin", arch: "arm64"})

	require.Equal(t, []string{
		"go", "build",
		"-trimpath",
		"-ldflags", "-s -w -X github.com/nicholasSUSE/cargueiro/internal/version.version=v0.1.0 -X github.com/nicholasSUSE/cargueiro/internal/version.commit=abc1234",
		"-o", filepath.Join("out", "cargueiro_darwin_arm64"),
		".",
	}, cmd.Args)
	require.Contains(t, cmd.Env, "CGO_ENABLED=0")
	require.Contains(t, cmd.Env, "GOOS=darwin")
	require.Contains(t, cmd.Env, "GOARCH=arm64")
}

// recorder is a runner that records the commands it is given and fails for the
// targets in failing.
type recorder struct {
	mu      sync.Mutex
	outputs []string
	failing map[string]error
}

func (r *recorder) run(cmd *exec.Cmd) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// The output file is the argument after -o.
	for i, arg := range cmd.Args {
		if arg == "-o" {
			r.outputs = append(r.outputs, cmd.Args[i+1])
			return r.failing[filepath.Base(cmd.Args[i+1])]
		}
	}
	return errors.New("no -o in the command")
}

func TestBuildAll(t *testing.T) {
	cfg := config{
		version: "dev",
		commit:  "none",
		outDir:  filepath.Join(t.TempDir(), "bin"),
	}
	rec := &recorder{}

	paths, err := buildAll(context.Background(), cfg, rec.run)

	require.NoError(t, err)
	want := []string{
		filepath.Join(cfg.outDir, "cargueiro_linux_amd64"),
		filepath.Join(cfg.outDir, "cargueiro_darwin_amd64"),
		filepath.Join(cfg.outDir, "cargueiro_darwin_arm64"),
		filepath.Join(cfg.outDir, "cargueiro_freebsd_amd64"),
	}
	require.Equal(t, want, paths)
	require.ElementsMatch(t, want, rec.outputs)
	require.DirExists(t, cfg.outDir)
}

func TestBuildAllReportsEveryFailure(t *testing.T) {
	cfg := config{
		version: "dev",
		commit:  "none",
		outDir:  t.TempDir(),
	}
	rec := &recorder{failing: map[string]error{
		"cargueiro_darwin_arm64":  errors.New("boom arm64"),
		"cargueiro_freebsd_amd64": errors.New("boom freebsd"),
	}}

	paths, err := buildAll(context.Background(), cfg, rec.run)

	require.ErrorContains(t, err, "build darwin/arm64: boom arm64")
	require.ErrorContains(t, err, "build freebsd/amd64: boom freebsd")
	require.Nil(t, paths)
	require.Len(t, rec.outputs, 4, "a failing target does not stop the others")
}
