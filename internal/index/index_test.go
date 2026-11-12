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

	"github.com/rancher/cargueiro/internal/rules"
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

// entryText returns the entry of the fleet chart in a version, pinned to digest,
// pushed on 2026-11-11.
func entryText(version, digest string) string {
	return entryTextCreated(version, digest, "2026-11-11T17:33:45Z")
}

// entryTextCreated is entryText for a chart pushed at created. An empty created
// leaves the entry without the date, as the entries published before it had.
func entryTextCreated(version, digest, created string) string {
	date := ""
	if created != "" {
		date = "created: \"" + created + "\"\n"
	}
	return `annotations:
  catalog.cattle.io/certified: rancher
apiVersion: v2
appVersion: 0.17.3
description: Fleet Controller
icon: file://assets/logos/fleet.svg
name: fleet
` + date + `urls:
- oci://` + testRegistry + `/fleet:` + strings.ReplaceAll(version, "+", "_") + `@` + digest + `
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
		_, err := Write(context.Background(), cfg, dir, opts)
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(dir, "assets", "fleet", "fleet-"+first+".tgz"))
		require.NoError(t, err)
		require.Equal(t, archiveFirst, got)
		index, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
		require.NoError(t, err)
		require.Contains(t, string(index), "digest: "+sha256Hex(archiveFirst)+"\n")
		require.Contains(t, string(index), "version: "+first+"\n", "the version keeps its +")
		require.Contains(t, string(index), "created: \"2026-11-11T17:33:45Z\"\n", "and the date its chart was pushed")
		require.Contains(t, string(index), "generated: \"2026-11-11T17:33:45Z\"\n", "which is also when the index was generated")
		require.Contains(t, string(index), "/fleet:111.0.3_up0.17.3@"+pinnedOne+"\n", "and the tag of its url is the one the registry stores")
		require.Len(t, fake.calls, 1)
	})

	t.Run("a published version is not pulled again", func(t *testing.T) {
		before, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
		require.NoError(t, err)
		fake.calls = nil

		_, err = Write(context.Background(), cfg, dir, opts)
		require.NoError(t, err)

		after, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(before), string(after))
		require.Empty(t, fake.calls, "nothing is new, so nothing is pulled")
	})

	t.Run("only the new version is pulled", func(t *testing.T) {
		writeFiles(t, dir, map[string]string{"entries/fleet/" + second + ".yaml": entryText(second, pinnedTwo)})
		fake.calls = nil

		_, err := Write(context.Background(), cfg, dir, opts)
		require.NoError(t, err)

		require.Len(t, fake.calls, 1)
		require.Equal(t, pinnedTwo, fake.calls[0].digest)
		require.FileExists(t, filepath.Join(dir, "assets", "fleet", "fleet-"+second+".tgz"))
	})
}

func TestWriteRefusesANewVersionWhoseUrlHasThePlus(t *testing.T) {
	const version = "111.0.3+up0.17.3"
	fake := &fakePull{archives: map[string][]byte{pinnedOne: archiveOf(t, chartYAMLOf(version))}}
	dir := t.TempDir()
	entry := strings.Replace(entryText(version, pinnedOne), "fleet:111.0.3_up0.17.3@", "fleet:"+version+"@", 1)
	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entry})

	_, err := Write(context.Background(), loadRules(t), dir, Options{Registry: testRegistry, Pull: fake.pull})

	require.ErrorContains(t, err, "must be oci://"+testRegistry+"/fleet:111.0.3_up0.17.3@sha256:<digest>")
	require.Empty(t, fake.calls, "nothing is pulled")
	require.NoDirExists(t, filepath.Join(dir, "assets"))
}

func TestWriteGeneratedIsTheNewestCreated(t *testing.T) {
	const (
		first  = "111.0.3+up0.17.3"
		second = "111.0.4+up0.17.4"
		third  = "111.0.5+up0.17.5"
	)
	fake := &fakePull{archives: map[string][]byte{
		pinnedOne: archiveOf(t, chartYAMLOf(first)),
		pinnedTwo: archiveOf(t, chartYAMLOf(second)),
	}}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"entries/fleet/" + first + ".yaml":  entryTextCreated(first, pinnedOne, "2026-11-11T17:33:45Z"),
		"entries/fleet/" + second + ".yaml": entryTextCreated(second, pinnedTwo, "2026-11-13T09:00:00Z"),
	})
	opts := Options{Registry: testRegistry, Pull: fake.pull}
	cfg := loadRules(t)

	read := func(t *testing.T) string {
		t.Helper()

		data, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
		require.NoError(t, err)
		return string(data)
	}

	t.Run("the newest date of the versions", func(t *testing.T) {
		_, err := Write(context.Background(), cfg, dir, opts)
		require.NoError(t, err)

		text := read(t)
		require.Contains(t, text, "generated: \"2026-11-13T09:00:00Z\"\n")
		require.Contains(t, text, "created: \"2026-11-11T17:33:45Z\"\n")
		require.Contains(t, text, "created: \"2026-11-13T09:00:00Z\"\n")
	})

	t.Run("regenerating gives the same file", func(t *testing.T) {
		before := read(t)

		_, err := Write(context.Background(), cfg, dir, opts)
		require.NoError(t, err)

		require.Equal(t, before, read(t))
	})

	t.Run("a hidden version does not count", func(t *testing.T) {
		writeFiles(t, dir, map[string]string{"entries/fleet/" + second + ".yaml": entryTextCreated(second, pinnedTwo, "2026-11-13T09:00:00Z") + "hide: true\n"})

		_, err := Write(context.Background(), cfg, dir, opts)
		require.NoError(t, err)

		text := read(t)
		require.Contains(t, text, "generated: \"2026-11-11T17:33:45Z\"\n", "the newest of the versions that are listed")
		require.NotContains(t, text, "2026-11-13T09:00:00Z")
	})

	t.Run("a new version changes only its line and generated", func(t *testing.T) {
		before := read(t)
		const pinnedThree = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
		fake.archives[pinnedThree] = archiveOf(t, chartYAMLOf(third))
		writeFiles(t, dir, map[string]string{"entries/fleet/" + third + ".yaml": entryTextCreated(third, pinnedThree, "2026-11-15T12:00:00Z")})

		_, err := Write(context.Background(), cfg, dir, opts)
		require.NoError(t, err)

		after := read(t)
		require.Contains(t, after, "generated: \"2026-11-15T12:00:00Z\"\n")
		require.NotContains(t, before, "2026-11-15T12:00:00Z")
		for _, line := range strings.Split(before, "\n") {
			if strings.HasPrefix(line, "generated:") {
				continue
			}
			require.Contains(t, after, line+"\n", "a line of the old index is still there")
		}
	})
}

func TestWriteRefusesANewVersionWithoutACreatedDate(t *testing.T) {
	const version = "111.0.3+up0.17.3"
	fake := &fakePull{archives: map[string][]byte{pinnedOne: archiveOf(t, chartYAMLOf(version))}}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entryTextCreated(version, pinnedOne, "")})

	_, err := Write(context.Background(), loadRules(t), dir, Options{Registry: testRegistry, Pull: fake.pull})

	require.ErrorContains(t, err, "fleet "+version+" has no created date: it must have the date its chart was pushed")
	require.Empty(t, fake.calls, "nothing is pulled")
	require.NoDirExists(t, filepath.Join(dir, "assets"))
	require.NoFileExists(t, filepath.Join(dir, "index.yaml"))
}

func TestWriteRefusesAChangedCreatedDateOfAPublishedVersion(t *testing.T) {
	const version = "111.0.3+up0.17.3"
	fake := &fakePull{archives: map[string][]byte{pinnedOne: archiveOf(t, chartYAMLOf(version))}}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entryText(version, pinnedOne)})
	opts := Options{Registry: testRegistry, Pull: fake.pull}
	_, err := Write(context.Background(), loadRules(t), dir, opts)
	require.NoError(t, err)
	before, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
	require.NoError(t, err)

	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entryTextCreated(version, pinnedOne, "2026-12-01T00:00:00Z")})
	_, err = Write(context.Background(), loadRules(t), dir, opts)

	require.ErrorContains(t, err, "fleet "+version+" is in index.yaml and its entry no longer matches it: a published version never changes")
	after, readErr := os.ReadFile(filepath.Join(dir, "index.yaml"))
	require.NoError(t, readErr)
	require.Equal(t, string(before), string(after))
}

func TestWriteKeepsAPublishedVersionThatHasNoCreatedDate(t *testing.T) {
	// The versions published before the entries had the date have none in the
	// entry, and the zero date in the index, and they stay as they are.
	const version = "111.0.3+up0.17.3"
	fake := &fakePull{archives: map[string][]byte{pinnedOne: archiveOf(t, chartYAMLOf(version))}}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entryText(version, pinnedOne)})
	opts := Options{Registry: testRegistry, Pull: fake.pull}
	_, err := Write(context.Background(), loadRules(t), dir, opts)
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(dir, "index.yaml"))
	require.NoError(t, err)
	old := strings.ReplaceAll(string(data), "\"2026-11-11T17:33:45Z\"", "\"0001-01-01T00:00:00Z\"")
	require.NotEqual(t, string(data), old)
	writeFiles(t, dir, map[string]string{
		"index.yaml":                         old,
		"entries/fleet/" + version + ".yaml": entryTextCreated(version, pinnedOne, ""),
	})
	fake.calls = nil

	_, err = Write(context.Background(), loadRules(t), dir, opts)

	require.NoError(t, err)
	require.Empty(t, fake.calls, "nothing is new")
	after, readErr := os.ReadFile(filepath.Join(dir, "index.yaml"))
	require.NoError(t, readErr)
	require.Equal(t, old, string(after), "and the index is the same")
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

	_, err := Write(context.Background(), loadRules(t), dir, Options{Registry: testRegistry, Pull: fake.pull})

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

	_, err := Write(context.Background(), loadRules(t), dir, Options{Registry: testRegistry, Pull: fake.pull})

	require.Error(t, err)
	require.Len(t, fake.calls, 1, "the archive was pulled before the index failed")
	require.NoFileExists(t, filepath.Join(dir, "assets", "fleet", "fleet-"+version+".tgz"), "the archive is taken back")
	require.NoDirExists(t, filepath.Join(dir, "assets", "fleet"))
}

func TestWriteReturnsWhatItDid(t *testing.T) {
	const version = "111.0.3+up0.17.3"
	fake := &fakePull{archives: map[string][]byte{pinnedOne: archiveOf(t, chartYAMLOf(version))}}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entryText(version, pinnedOne)})
	cfg := loadRules(t)
	opts := Options{Registry: testRegistry, Pull: fake.pull}

	plan, err := Write(context.Background(), cfg, dir, opts)
	require.NoError(t, err)
	require.Equal(t, Plan{Add: []Ref{{Chart: "fleet", Version: version}}}, plan)

	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entryText(version, pinnedOne) + "hide: true\n"})
	plan, err = Write(context.Background(), cfg, dir, opts)
	require.NoError(t, err)
	require.Equal(t, Plan{Hide: []Ref{{Chart: "fleet", Version: version}}}, plan)
}

func TestWriteDryRun(t *testing.T) {
	const version = "111.0.3+up0.17.3"
	fake := &fakePull{archives: map[string][]byte{pinnedOne: archiveOf(t, chartYAMLOf(version))}}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": entryText(version, pinnedOne)})

	plan, err := Write(context.Background(), loadRules(t), dir, Options{Registry: testRegistry, Pull: fake.pull, DryRun: true})

	require.NoError(t, err)
	require.Equal(t, Plan{Add: []Ref{{Chart: "fleet", Version: version}}}, plan)
	require.Empty(t, fake.calls, "nothing is pulled")
	require.NoFileExists(t, filepath.Join(dir, "index.yaml"))
	require.NoDirExists(t, filepath.Join(dir, "assets"))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "only entries/ is there")
}

func TestWriteDryRunStillChecks(t *testing.T) {
	const version = "111.0.3+up0.17.3"
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"entries/fleet/" + version + ".yaml": strings.Replace(entryText(version, pinnedOne), "@"+pinnedOne, "", 1)})

	plan, err := Write(context.Background(), loadRules(t), dir, Options{Registry: testRegistry, DryRun: true})

	require.ErrorContains(t, err, "fleet "+version+" has the url")
	require.Equal(t, Plan{}, plan)
}
