// Package archivetest compares chart archives by content instead of bytes.
//
// A .tgz built by different Helm versions differs in its gzip header, and tar
// headers carry file mtimes that change on every checkout. A manifest keeps
// only what identifies the content: mode, owner, size and sha256 per file.
package archivetest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Fataler is the subset of testing.TB used by the assertions.
type Fataler interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Manifest reads a gzipped tar stream and returns one line per regular file,
// sorted by name:
//
//	<mode> <uid>:<gid> <size> <sha256> <name>
//
// Directory entries are skipped. Any other entry type is an error.
func Manifest(r io.Reader) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("open gzip stream: %w", err)
	}
	defer func() { _ = gz.Close() }() // read-only: Close has nothing to flush

	var lines []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar entry: %w", err)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return "", fmt.Errorf("entry %s: unsupported type %q", hdr.Name, hdr.Typeflag)
		}

		sum := sha256.New()
		size, err := io.Copy(sum, tr)
		if err != nil {
			return "", fmt.Errorf("hash entry %s: %w", hdr.Name, err)
		}
		lines = append(lines, fmt.Sprintf("%04o %d:%d %d %s %s",
			hdr.Mode&0o7777, hdr.Uid, hdr.Gid, size, hex.EncodeToString(sum.Sum(nil)), hdr.Name))
	}

	sort.Slice(lines, func(i, j int) bool { return entryName(lines[i]) < entryName(lines[j]) })
	return strings.Join(lines, "\n") + "\n", nil
}

// entryName returns the name field of a manifest line.
func entryName(line string) string {
	fields := strings.SplitN(line, " ", 5)
	return fields[len(fields)-1]
}

// ManifestFile is Manifest for an archive on disk.
func ManifestFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open archive %s: %w", path, err)
	}
	defer func() { _ = f.Close() }() // read-only: Close has nothing to flush

	return Manifest(f)
}

// AssertManifest fails t unless the content of archivePath matches the
// manifest stored in goldenPath. Golden manifests are never rewritten here.
func AssertManifest(t Fataler, archivePath, goldenPath string) {
	t.Helper()

	got, err := ManifestFile(archivePath)
	if err != nil {
		t.Fatalf("%v", err)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden manifest %s: %v", goldenPath, err)
		return
	}

	if !bytes.Equal(want, []byte(got)) {
		t.Fatalf("archive %s does not match golden manifest %s\n\nWANT:\n%s\nGOT:\n%s", archivePath, goldenPath, want, got)
	}
}
