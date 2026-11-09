package oci

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
)

// fakeClient records the pushes it receives.
type fakeClient struct {
	refs []string
	data [][]byte
	err  error

	// stored are the references Exists reports as present.
	stored []string
	// existsErr is returned by Exists.
	existsErr error
	// checked are the references Exists was asked about.
	checked []string
}

func (f *fakeClient) Exists(ref string) (bool, error) {
	f.checked = append(f.checked, ref)
	return slices.Contains(f.stored, ref), f.existsErr
}

func (f *fakeClient) Push(data []byte, ref string) (string, error) {
	f.refs = append(f.refs, ref)
	f.data = append(f.data, data)
	if f.err != nil {
		return "", f.err
	}
	return "stored/" + ref, nil
}

// writeChart packages a minimal chart with the given name and version and
// returns the archive path.
func writeChart(t *testing.T, name, version string) string {
	t.Helper()

	src := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.MkdirAll(filepath.Join(src, "templates"), 0o755))
	meta := "apiVersion: v2\nname: " + name + "\nversion: " + version + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(src, "Chart.yaml"), []byte(meta), 0o600))

	archive, err := chart.Package(context.Background(), src, t.TempDir())
	require.NoError(t, err)
	return archive
}

func TestPusherPush(t *testing.T) {
	target := Target{Host: "reg.local:5000", Path: "team"}

	t.Run("reference comes from the chart metadata", func(t *testing.T) {
		archive := writeChart(t, "fleet", "1.2.3")
		fake := &fakeClient{}

		got, err := NewPusher(fake, target, newTestGuard(t)).Push(context.Background(), archive)

		require.NoError(t, err)
		require.Equal(t, []string{"reg.local:5000/team/fleet:1.2.3"}, fake.refs)
		require.Equal(t, "stored/reg.local:5000/team/fleet:1.2.3", got, "returns the client's reference")
	})

	t.Run("build metadata keeps the plus sign", func(t *testing.T) {
		archive := writeChart(t, "fleet", "110.0.3+up0.16.3")
		fake := &fakeClient{}

		_, err := NewPusher(fake, target, newTestGuard(t)).Push(context.Background(), archive)

		require.NoError(t, err)
		require.Equal(t, []string{"reg.local:5000/team/fleet:110.0.3+up0.16.3"}, fake.refs)
	})

	t.Run("file name does not matter", func(t *testing.T) {
		archive := writeChart(t, "fleet", "1.2.3")
		renamed := filepath.Join(t.TempDir(), "something-else.tgz")
		require.NoError(t, os.Rename(archive, renamed))
		fake := &fakeClient{}

		_, err := NewPusher(fake, target, newTestGuard(t)).Push(context.Background(), renamed)

		require.NoError(t, err)
		require.Equal(t, []string{"reg.local:5000/team/fleet:1.2.3"}, fake.refs)
	})

	t.Run("archive bytes are sent unchanged", func(t *testing.T) {
		archive := writeChart(t, "fleet", "1.2.3")
		want, err := os.ReadFile(archive)
		require.NoError(t, err)
		fake := &fakeClient{}

		_, err = NewPusher(fake, target, newTestGuard(t)).Push(context.Background(), archive)

		require.NoError(t, err)
		require.Equal(t, [][]byte{want}, fake.data)
	})

	t.Run("target without path", func(t *testing.T) {
		archive := writeChart(t, "fleet", "1.2.3")
		fake := &fakeClient{}

		_, err := NewPusher(fake, Target{Host: "reg.local"}, newTestGuard(t)).Push(context.Background(), archive)

		require.NoError(t, err)
		require.Equal(t, []string{"reg.local/fleet:1.2.3"}, fake.refs)
	})
}

func TestPusherPushFailures(t *testing.T) {
	target := Target{Host: "reg.local"}
	errBoom := errors.New("boom")

	notAnArchive := filepath.Join(t.TempDir(), "plain.tgz")
	require.NoError(t, os.WriteFile(notAnArchive, []byte("not a chart"), 0o600))

	tests := []struct {
		name    string
		archive func(t *testing.T) string
		client  *fakeClient
		wantIs  error
		wantMsg string
	}{
		{
			name:    "missing file",
			archive: func(*testing.T) string { return filepath.Join(t.TempDir(), "absent.tgz") },
			client:  &fakeClient{},
			wantIs:  os.ErrNotExist,
		},
		{
			name:    "not an archive",
			archive: func(*testing.T) string { return notAnArchive },
			client:  &fakeClient{},
			wantMsg: "invalid chart archive: ",
		},
		{
			name:    "chart name is not a valid repository name",
			archive: func(t *testing.T) string { return writeChart(t, "UpperCase", "1.0.0") },
			client:  &fakeClient{},
			wantMsg: "not a valid registry repository name",
		},
		{
			name:    "registry rejects the push",
			archive: func(t *testing.T) string { return writeChart(t, "fleet", "1.0.0") },
			client:  &fakeClient{err: errBoom},
			wantIs:  errBoom,
			wantMsg: "push ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive := tt.archive(t)

			got, err := NewPusher(tt.client, target, newTestGuard(t)).Push(context.Background(), archive)

			if tt.wantIs != nil {
				require.ErrorIs(t, err, tt.wantIs)
			}
			if tt.wantMsg != "" {
				require.ErrorContains(t, err, tt.wantMsg)
			}
			require.Empty(t, got)
			if !errors.Is(tt.wantIs, errBoom) {
				require.Empty(t, tt.client.refs, "nothing may be sent for an unusable archive")
			}
		})
	}
}

func TestPusherPushCanceledContext(t *testing.T) {
	archive := writeChart(t, "fleet", "1.0.0")
	fake := &fakeClient{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewPusher(fake, Target{Host: "reg.local"}, newTestGuard(t)).Push(ctx, archive)

	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, fake.refs)
}
