// Command build builds cargueiro for every release target. The binaries are
// written to bin/, next to the host build that make build writes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

const module = "github.com/nicholasSUSE/cargueiro"

// target is an operating system and an architecture cargueiro is released for.
type target struct {
	os   string
	arch string
}

// targets lists what a release is built for.
func targets() []target {
	return []target{
		{os: "linux", arch: "amd64"},
		{os: "darwin", arch: "amd64"},
		{os: "darwin", arch: "arm64"},
		{os: "freebsd", arch: "amd64"},
	}
}

func (t target) String() string {
	return t.os + "/" + t.arch
}

// binary is the file name of the build for t.
func (t target) binary() string {
	return "cargueiro_" + t.os + "_" + t.arch
}

type config struct {
	version string
	outDir  string
}

// command is the go build of t. It has no cgo, so the binary does not depend on
// the host it was built on, and it asks for no VCS stamp and trims the build
// path, so the bytes depend on the source only.
func (c config) command(ctx context.Context, t target) *exec.Cmd {
	ldflags := fmt.Sprintf("-s -w -X %s/internal/version.version=%s", module, c.version)

	cmd := exec.CommandContext(ctx, "go", "build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags", ldflags,
		"-o", filepath.Join(c.outDir, t.binary()),
		".",
	)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+t.os, "GOARCH="+t.arch)
	return cmd
}

// runner runs a command and reports what went wrong, with its output.
type runner func(cmd *exec.Cmd) error

func run(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}

// buildAll builds every target at the same time and returns the paths of the
// binaries in the order of targets. A target that fails does not stop the
// others, and every failure is reported.
func buildAll(ctx context.Context, cfg config, run runner) ([]string, error) {
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", cfg.outDir, err)
	}

	all := targets()
	errs := make([]error, len(all))

	var wg sync.WaitGroup
	for i, t := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := run(cfg.command(ctx, t)); err != nil {
				errs[i] = fmt.Errorf("build %s: %w", t, err)
			}
		}()
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	paths := make([]string, len(all))
	for i, t := range all {
		paths[i] = filepath.Join(cfg.outDir, t.binary())
	}
	return paths, nil
}

func main() {
	var cfg config
	flag.StringVar(&cfg.version, "version", "dev", "version to stamp into the binaries (dev unless this is a release)")
	flag.StringVar(&cfg.outDir, "out", "bin", "directory to write the binaries to")
	flag.Parse()

	paths, err := buildAll(context.Background(), cfg, run)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
	for _, path := range paths {
		fmt.Println(path)
	}
}
