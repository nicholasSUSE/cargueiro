//go:build install

// Package installing checks install.sh against a local fake release: a
// stand-in binary and a checksums.txt, served over HTTP. It redirects the
// script's download source and install directory with the internal
// CARGUEIRO_INSTALL_BASE_URL and CARGUEIRO_BIN_DIR variables, so it installs
// nothing on the host and needs no network or sudo.
package installing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// asset is the release file name for the host, the one the script downloads.
func asset(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("cargueiro_%s_%s", runtime.GOOS, runtime.GOARCH)
}

// scriptPath is tools/scripts/install.sh, seen from this package.
func scriptPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "tools", "scripts", "install.sh"))
	require.NoError(t, err)
	return path
}

// fakeBinary is a stand-in for cargueiro: it prints version when run with the
// version argument, so the script's final "cargueiro version" works.
func fakeBinary(version string) []byte {
	return []byte("#!/bin/sh\n[ \"$1\" = version ] && echo " + version + "\n")
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// release writes a fake release (the host asset and a checksums.txt) into dir
// and returns an HTTP server that serves that dir. checksum is the content of
// checksums.txt, so a test can serve a wrong one.
func release(t *testing.T, binary []byte, checksums string) *httptest.Server {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, asset(t)), binary, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(checksums), 0o644))

	srv := httptest.NewServer(http.FileServer(http.Dir(dir)))
	t.Cleanup(srv.Close)
	return srv
}

// run executes install.sh with the given extra environment, and returns its
// combined output and error.
func run(t *testing.T, binDir, baseURL string, extra ...string) (string, error) {
	t.Helper()

	cmd := exec.Command("sh", scriptPath(t))
	cmd.Env = append(os.Environ(),
		"CARGUEIRO_INSTALL_BASE_URL="+baseURL,
		"CARGUEIRO_BIN_DIR="+binDir,
	)
	cmd.Env = append(cmd.Env, extra...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInstall(t *testing.T) {
	binary := fakeBinary("v9.9.9")
	checksums := fmt.Sprintf("%s  %s\n", sum(binary), asset(t))
	srv := release(t, binary, checksums)

	binDir := t.TempDir()
	out, err := run(t, binDir, srv.URL)
	require.NoError(t, err, out)

	installed := filepath.Join(binDir, "cargueiro")
	info, statErr := os.Stat(installed)
	require.NoError(t, statErr, "cargueiro was not installed")
	require.NotZero(t, info.Mode()&0o111, "installed cargueiro is not executable")
	require.Contains(t, out, "v9.9.9", "the script did not run cargueiro version")
}

func TestInstallPinnedVersion(t *testing.T) {
	binary := fakeBinary("v1.2.3")
	checksums := fmt.Sprintf("%s  %s\n", sum(binary), asset(t))
	srv := release(t, binary, checksums)

	binDir := t.TempDir()
	// The base URL is already pinned here, but setting CARGUEIRO_VERSION proves
	// the script accepts the pin and still installs.
	out, err := run(t, binDir, srv.URL, "CARGUEIRO_VERSION=v1.2.3")
	require.NoError(t, err, out)

	require.FileExists(t, filepath.Join(binDir, "cargueiro"))
	require.Contains(t, out, "v1.2.3")
}

func TestWrongChecksumInstallsNothing(t *testing.T) {
	binary := fakeBinary("v9.9.9")
	// A checksum that does not match the binary.
	checksums := fmt.Sprintf("%s  %s\n", sum([]byte("something else")), asset(t))
	srv := release(t, binary, checksums)

	binDir := t.TempDir()
	out, err := run(t, binDir, srv.URL)
	require.Error(t, err, "a wrong checksum must fail the install")
	require.Contains(t, out, "checksum mismatch")

	_, statErr := os.Stat(filepath.Join(binDir, "cargueiro"))
	require.ErrorIs(t, statErr, os.ErrNotExist, "a wrong checksum must install nothing")
}
