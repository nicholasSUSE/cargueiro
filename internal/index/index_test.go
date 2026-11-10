package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/rules"
)

const (
	testRegistry = "registry.example.com/rancher"
	pinnedOne    = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	pinnedTwo    = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
)

func loadRules(t *testing.T) *rules.Config {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "rules.yaml"))
	require.NoError(t, err)
	cfg, err := rules.Parse(data)
	require.NoError(t, err)
	return cfg
}

// chartYAMLOf returns the Chart.yaml of the fleet chart in a version.
func chartYAMLOf(version string) string {
	return strings.Replace(fleetChartYAML, "111.0.3+up0.17.3", version, 1)
}

// entryText returns the entry of the fleet chart in a version, pinned to digest.
func entryText(version, digest string) string {
	return `annotations:
  catalog.cattle.io/certified: rancher
apiVersion: v2
appVersion: 0.17.3
description: Fleet Controller
icon: file://assets/logos/fleet.svg
name: fleet
urls:
- oci://` + testRegistry + `/fleet:` + version + `@` + digest + `
version: ` + version + `
`
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestWrite(t *testing.T) {
	const (
		first  = "111.0.3+up0.17.3"
		second = "111.0.4+up0.17.4"
	)
	archiveFirst := archiveOf(t, chartYAMLOf(first))
	archiveSecond := archiveOf(t, chartYAMLOf(second))
	fake := &fakePull{archives: map[string][]byte{pinnedOne: archiveFirst, pinnedTwo: archiveSecond}}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"entries/fleet/" + first + ".yaml": entryText(first, pinnedOne)})
	opts := Options{Registry: testRegistry, Pull: fake.pull}
	cfg := loadRules(t)

	t.Run("a new version is pulled, written and indexed with the digest of its archive", func(t *testing.T) {
		require.NoError(t, Write(context.Background(), cfg, dir, opts))

		got, err := os.ReadFile(filepath.Join(dir, "assets", "fleet", "fleet-"+first+".tgz"))
		require.NoError(t, err)
		require.Equal(t, archiveFirst, got)
		index, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
		require.NoError(t, err)
		require.Contains(t, string(index), "digest: "+sha256Hex(archiveFirst)+"\n")
		require.Len(t, fake.calls, 1)
	})

	t.Run("a published version is not pulled again", func(t *testing.T) {
		before, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
		require.NoError(t, err)
		fake.calls = nil

		require.NoError(t, Write(context.Background(), cfg, dir, opts))

		after, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(before), string(after))
		require.Empty(t, fake.calls, "nothing is new, so nothing is pulled")
	})

	t.Run("only the new version is pulled", func(t *testing.T) {
		writeFiles(t, dir, map[string]string{"entries/fleet/" + second + ".yaml": entryText(second, pinnedTwo)})
		fake.calls = nil

		require.NoError(t, Write(context.Background(), cfg, dir, opts))

		require.Len(t, fake.calls, 1)
		require.Equal(t, pinnedTwo, fake.calls[0].digest)
		require.FileExists(t, filepath.Join(dir, "assets", "fleet", "fleet-"+second+".tgz"))
	})
}

func TestWriteWritesNothingWhenAPullFails(t *testing.T) {
	const (
		first  = "111.0.3+up0.17.3"
		second = "111.0.4+up0.17.4"
	)
	fake := &fakePull{
		archives: map[string][]byte{pinnedOne: archiveOf(t, chartYAMLOf(first))},
		failures: map[string]error{pinnedTwo: errors.New("the registry is down")},
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"entries/fleet/" + first + ".yaml":  entryText(first, pinnedOne),
		"entries/fleet/" + second + ".yaml": entryText(second, pinnedTwo),
	})

	err := Write(context.Background(), loadRules(t), dir, Options{Registry: testRegistry, Pull: fake.pull})

	require.ErrorContains(t, err, "fleet "+second+": pull the archive: the registry is down")
	require.NoDirExists(t, filepath.Join(dir, "assets"), "the version that was pulled is not kept either")
	require.NoFileExists(t, filepath.Join(dir, "index.yaml"))
}

func TestWriteTakesTheArchivesBackWhenTheIndexCannotBeWritten(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a folder cannot be made read-only for root")
	}

	const version = "111.0.3+up0.17.3"
	fake := &fakePull{archives: map[string][]byte{pinnedOne: archiveOf(t, chartYAMLOf(version))}}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entryText(version, pinnedOne)})
	// assets/ can be written, but the repository folder cannot, so the archive
	// is written and then index.yaml is not.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o750))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := Write(context.Background(), loadRules(t), dir, Options{Registry: testRegistry, Pull: fake.pull})

	require.Error(t, err)
	require.Len(t, fake.calls, 1, "the archive was pulled before the index failed")
	require.NoFileExists(t, filepath.Join(dir, "assets", "fleet", "fleet-"+version+".tgz"), "the archive is taken back")
	require.NoDirExists(t, filepath.Join(dir, "assets", "fleet"))
}
