package archivetest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type entry struct {
	name     string
	typeflag byte
	mode     int64
	body     string
}

// buildTgz returns a gzipped tar of entries with every file stamped at mtime.
// comment sets the gzip header comment, which Manifest must ignore.
func buildTgz(t *testing.T, entries []entry, mtime time.Time, comment string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Comment = comment
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typeflag := e.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Typeflag: typeflag, Mode: e.mode, ModTime: mtime}
		if typeflag == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if typeflag == tar.TypeSymlink {
			hdr.Linkname = "target"
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func sampleEntries() []entry {
	return []entry{
		{name: "chart/values.yaml", mode: 0o644, body: "a: 1\n"},
		{name: "chart/Chart.yaml", mode: 0o644, body: "name: chart\n"},
		{name: "chart/templates", typeflag: tar.TypeDir, mode: 0o755},
	}
}

func TestManifest(t *testing.T) {
	now := time.Now()

	t.Run("sorted, dirs skipped, one line per file", func(t *testing.T) {
		got, err := Manifest(bytes.NewReader(buildTgz(t, sampleEntries(), now, "")))
		require.NoError(t, err)

		require.Regexp(t, `^0644 0:0 12 [0-9a-f]{64} chart/Chart\.yaml\n0644 0:0 5 [0-9a-f]{64} chart/values\.yaml\n$`, got)
	})

	t.Run("ignores mtime and gzip header", func(t *testing.T) {
		a, err := Manifest(bytes.NewReader(buildTgz(t, sampleEntries(), now, "Helm")))
		require.NoError(t, err)
		b, err := Manifest(bytes.NewReader(buildTgz(t, sampleEntries(), now.Add(-72*time.Hour), "other")))
		require.NoError(t, err)

		require.Equal(t, a, b)
	})

	t.Run("entry order does not matter", func(t *testing.T) {
		reversed := []entry{sampleEntries()[1], sampleEntries()[0]}
		a, err := Manifest(bytes.NewReader(buildTgz(t, sampleEntries(), now, "")))
		require.NoError(t, err)
		b, err := Manifest(bytes.NewReader(buildTgz(t, reversed, now, "")))
		require.NoError(t, err)

		require.Equal(t, a, b)
	})

	t.Run("content change is detected", func(t *testing.T) {
		changed := []entry{{name: "chart/values.yaml", mode: 0o644, body: "a: 2\n"}}
		original := []entry{{name: "chart/values.yaml", mode: 0o644, body: "a: 1\n"}}
		a, err := Manifest(bytes.NewReader(buildTgz(t, original, now, "")))
		require.NoError(t, err)
		b, err := Manifest(bytes.NewReader(buildTgz(t, changed, now, "")))
		require.NoError(t, err)

		require.NotEqual(t, a, b)
	})

	t.Run("mode change is detected", func(t *testing.T) {
		a, err := Manifest(bytes.NewReader(buildTgz(t, []entry{{name: "f", mode: 0o644}}, now, "")))
		require.NoError(t, err)
		b, err := Manifest(bytes.NewReader(buildTgz(t, []entry{{name: "f", mode: 0o755}}, now, "")))
		require.NoError(t, err)

		require.NotEqual(t, a, b)
	})

	t.Run("symlink is rejected", func(t *testing.T) {
		_, err := Manifest(bytes.NewReader(buildTgz(t, []entry{{name: "link", typeflag: tar.TypeSymlink}}, now, "")))
		require.ErrorContains(t, err, "unsupported type")
	})

	t.Run("not gzip", func(t *testing.T) {
		_, err := Manifest(bytes.NewReader([]byte("plain text")))
		require.ErrorContains(t, err, "open gzip stream")
	})
}

type fakeT struct {
	failed bool
	msg    string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Fatalf(format string, args ...any) {
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
}

func TestAssertManifest(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "chart.tgz")
	require.NoError(t, os.WriteFile(archive, buildTgz(t, sampleEntries(), time.Now(), ""), 0o600))

	want, err := ManifestFile(archive)
	require.NoError(t, err)

	tests := []struct {
		name       string
		archive    string
		golden     string
		wantFailed bool
	}{
		{name: "matching golden", archive: archive, golden: "match"},
		{name: "different golden", archive: archive, golden: "differ", wantFailed: true},
		{name: "missing golden", archive: archive, golden: "absent", wantFailed: true},
		{name: "missing archive", archive: filepath.Join(dir, "nope.tgz"), golden: "match", wantFailed: true},
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "match"), []byte(want), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "differ"), []byte("0644 0:0 1 x y\n"), 0o600))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := &fakeT{}
			AssertManifest(ft, tt.archive, filepath.Join(dir, tt.golden))
			require.Equal(t, tt.wantFailed, ft.failed, ft.msg)
		})
	}
}
