package integrity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

// helloDigest is the sha256 of the text "hello\n", as sha256sum prints it.
const helloDigest = digest.Digest("sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03")

func TestFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chart.tgz")
	require.NoError(t, os.WriteFile(path, []byte("hello\n"), 0o600))

	got, err := File(path)

	require.NoError(t, err)
	require.Equal(t, helloDigest, got)
}

func TestFileOfAnEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	require.NoError(t, os.WriteFile(path, nil, 0o600))

	got, err := File(path)

	require.NoError(t, err)
	require.Equal(t, digest.Digest("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"), got)
}

func TestFileThatDoesNotExist(t *testing.T) {
	got, err := File(filepath.Join(t.TempDir(), "nope"))

	require.ErrorContains(t, err, "digest ")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Empty(t, got)
}

func TestBytesIsTheDigestOfTheFile(t *testing.T) {
	require.Equal(t, helloDigest, Bytes([]byte("hello\n")))
}

func manifest(t *testing.T, entries map[string]string) *Manifest {
	t.Helper()

	m := NewManifest()
	for name, content := range entries {
		require.NoError(t, m.Add(name, Bytes([]byte(content))))
	}
	return m
}

func TestManifestAdd(t *testing.T) {
	t.Run("an entry", func(t *testing.T) {
		m := NewManifest()

		require.NoError(t, m.Add("Chart.yaml", helloDigest))
		require.Equal(t, 1, m.Len())
	})

	t.Run("a name twice", func(t *testing.T) {
		m := NewManifest()
		require.NoError(t, m.Add("Chart.yaml", helloDigest))

		err := m.Add("Chart.yaml", Bytes([]byte("other")))

		require.EqualError(t, err, `"Chart.yaml" is in the manifest twice`)
		require.Equal(t, 1, m.Len())
	})

	t.Run("no name", func(t *testing.T) {
		require.EqualError(t, NewManifest().Add("", helloDigest), "an entry of the manifest needs a name")
	})

	t.Run("a digest that is not one", func(t *testing.T) {
		err := NewManifest().Add("a", digest.Digest("not-a-digest"))

		require.ErrorContains(t, err, `digest of "a"`)
	})
}

func TestLenOfNil(t *testing.T) {
	var m *Manifest

	require.Zero(t, m.Len())
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a    map[string]string
		b    map[string]string
		want Differences
	}{
		{
			name: "the same",
			a:    map[string]string{"Chart.yaml": "one", "values.yaml": "two"},
			b:    map[string]string{"values.yaml": "two", "Chart.yaml": "one"},
			want: nil,
		},
		{
			name: "both empty",
			a:    nil,
			b:    nil,
			want: nil,
		},
		{
			name: "a file with other bytes",
			a:    map[string]string{"Chart.yaml": "one", "values.yaml": "two"},
			b:    map[string]string{"Chart.yaml": "one", "values.yaml": "TWO"},
			want: Differences{{Name: "values.yaml", Kind: Changed, A: Bytes([]byte("two")), B: Bytes([]byte("TWO"))}},
		},
		{
			name: "a file only in the first",
			a:    map[string]string{"Chart.yaml": "one", "README.md": "readme"},
			b:    map[string]string{"Chart.yaml": "one"},
			want: Differences{{Name: "README.md", Kind: OnlyInA, A: Bytes([]byte("readme"))}},
		},
		{
			name: "a file only in the second",
			a:    map[string]string{"Chart.yaml": "one"},
			b:    map[string]string{"Chart.yaml": "one", "NOTES.txt": "notes"},
			want: Differences{{Name: "NOTES.txt", Kind: OnlyInB, B: Bytes([]byte("notes"))}},
		},
		{
			name: "everything at once, sorted by name",
			a:    map[string]string{"b": "1", "d": "x", "a": "same"},
			b:    map[string]string{"c": "3", "d": "y", "a": "same"},
			want: Differences{
				{Name: "b", Kind: OnlyInA, A: Bytes([]byte("1"))},
				{Name: "c", Kind: OnlyInB, B: Bytes([]byte("3"))},
				{Name: "d", Kind: Changed, A: Bytes([]byte("x")), B: Bytes([]byte("y"))},
			},
		},
		{
			name: "the first is empty",
			a:    nil,
			b:    map[string]string{"a": "1"},
			want: Differences{{Name: "a", Kind: OnlyInB, B: Bytes([]byte("1"))}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(manifest(t, tt.a), manifest(t, tt.b))

			require.Equal(t, tt.want, got)
		})
	}
}

func TestCompareWithNilManifests(t *testing.T) {
	require.Empty(t, Compare(nil, nil))
	require.Equal(t, Differences{{Name: "a", Kind: OnlyInB, B: Bytes([]byte("1"))}},
		Compare(nil, manifest(t, map[string]string{"a": "1"})))
	require.Equal(t, Differences{{Name: "a", Kind: OnlyInA, A: Bytes([]byte("1"))}},
		Compare(manifest(t, map[string]string{"a": "1"}), nil))
}

func TestCompareDoesNotChangeTheManifests(t *testing.T) {
	a := manifest(t, map[string]string{"a": "1", "b": "2"})
	b := manifest(t, map[string]string{"a": "1"})

	Compare(a, b)

	require.Equal(t, 2, a.Len())
	require.Equal(t, 1, b.Len())
}

func TestDifferencesErr(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		require.NoError(t, Differences(nil).Err("directory", "archive"))
	})

	t.Run("each file on its own line, named after what was compared", func(t *testing.T) {
		diffs := Compare(
			manifest(t, map[string]string{"b": "1", "d": "x", "a": "same"}),
			manifest(t, map[string]string{"c": "3", "d": "y", "a": "same"}),
		)

		err := diffs.Err("directory", "archive")

		require.EqualError(t, err, "3 files differ between the directory and the archive:\n"+
			"  b: only in the directory\n"+
			"  c: only in the archive\n"+
			"  d: the directory has "+string(Bytes([]byte("x")))+" and the archive has "+string(Bytes([]byte("y"))))
	})
}
