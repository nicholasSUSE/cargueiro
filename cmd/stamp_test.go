package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// stampChartYAML is a chart as the owner's automation leaves it: an upstream
// version, and no helm.sh/images. checkRules lists it as a main chart that needs
// the annotation certified, and ties Rancher 2.16 to the major 111.
const stampChartYAML = `apiVersion: v2
name: fleet
version: 0.16.3
appVersion: 0.16.3
annotations:
  catalog.cattle.io/certified: rancher
  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'
`

const stampValuesYAML = `image:
  repository: rancher/fleet
  tag: v0.16.3
agent:
  repository: rancher/fleet-agent
  tag: v0.16.3
`

// runStamp executes stamp with the rules in checkRules and a registry that must
// not be asked.
func runStamp(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	return runStampWith(t, &fakeLister{err: errors.New("the registry must not be asked")}, args...)
}

// runStampWith is runStamp with the registry given.
func runStampWith(t *testing.T, fake *fakeLister, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{listTags: fake.list})
	require.NoError(t, err)
	root.SetArgs(append([]string{"stamp"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestStamp(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)

	stdout, stderr, err := runStamp(t, dir, "--version", "111.0.0+up0.16.3")

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "version: 0.16.3 -> 111.0.0+up0.16.3\nhelm.sh/images: added fleet-agent, fleet\n", stdout)

	written, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.Equal(t, strings.Replace(stampChartYAML, "version: 0.16.3", "version: 111.0.0+up0.16.3", 1)+
		"  helm.sh/images: |\n    - name: fleet-agent\n      image: rancher/fleet-agent:v0.16.3\n    - name: fleet\n      image: rancher/fleet:v0.16.3\n",
		string(written))
}

func TestStampTwice(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)
	_, _, err := runStamp(t, dir, "--version", "111.0.0+up0.16.3")
	require.NoError(t, err)

	stdout, stderr, err := runStamp(t, dir, "--version", "111.0.0+up0.16.3")

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "nothing to change\n", stdout)
}

func TestStampOnlyTheVersionChanges(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, "")

	stdout, _, err := runStamp(t, dir, "--version", "111.0.1+up0.16.3")

	require.NoError(t, err)
	require.Equal(t, "version: 0.16.3 -> 111.0.1+up0.16.3\n", stdout)
}

func TestStampRefusesAndChangesNothing(t *testing.T) {
	tests := []struct {
		name       string
		chartYAML  string
		valuesYAML string
		args       []string
		wantErr    string
	}{
		{
			name:       "a version without a +up part",
			chartYAML:  stampChartYAML,
			valuesYAML: stampValuesYAML,
			args:       []string{"--version", "111.0.0"},
			wantErr:    `version "111.0.0" must look like 1XY.Z.W+upA.B.C`,
		},
		{
			name:       "the major of another Rancher version",
			chartYAML:  stampChartYAML,
			valuesYAML: stampValuesYAML,
			args:       []string{"--version", "112.0.0+up0.16.3"},
			wantErr:    "has major 112, but charts for Rancher 2.16 have major 111",
		},
		{
			name:       "an image with the tag latest",
			chartYAML:  stampChartYAML,
			valuesYAML: "image:\n  repository: rancher/fleet\n  tag: latest\n",
			args:       []string{"--version", "111.0.0+up0.16.3"},
			wantErr:    `values.yaml image: "tag" must not be "latest"`,
		},
		{
			name:       "a chart for Rancher 2.15",
			chartYAML:  strings.Replace(stampChartYAML, ">= 2.16.0-0 < 2.17.0-0", ">= 2.15.0-0 < 2.16.0-0", 1),
			valuesYAML: stampValuesYAML,
			args:       []string{"--version", "110.0.0+up0.16.3"},
			wantErr:    "Rancher 2.16 and later are supported, 2.15 and before are not",
		},
		{
			name:       "a chart that is not in the rules file",
			chartYAML:  strings.Replace(stampChartYAML, "name: fleet", "name: epinio", 1),
			valuesYAML: stampValuesYAML,
			args:       []string{"--version", "111.0.0+up0.16.3"},
			wantErr:    `chart "epinio" is not listed in the rules file`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeChartDir(t, tt.chartYAML, tt.valuesYAML)

			stdout, _, err := runStamp(t, append([]string{dir}, tt.args...)...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			onDisk, readErr := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
			require.NoError(t, readErr)
			require.Equal(t, tt.chartYAML, string(onDisk), "nothing is written when a check fails")
		})
	}
}

func TestStampNeedsADirectory(t *testing.T) {
	t.Run("no directory", func(t *testing.T) {
		_, _, err := runStamp(t, "--version", "111.0.0+up0.16.3")
		require.ErrorContains(t, err, "accepts 1 arg(s), received 0")
	})

	t.Run("a directory that does not exist", func(t *testing.T) {
		_, _, err := runStamp(t, filepath.Join(t.TempDir(), "nope"), "--version", "111.0.0+up0.16.3")
		require.ErrorContains(t, err, "Chart.yaml not found")
	})
}

func TestStampWorksOutTheVersion(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)
	fake := &fakeLister{tags: []string{"111.0.0_up0.16.1", "111.0.1_up0.16.2"}}

	stdout, stderr, err := runStampWith(t, fake, dir)

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "version: 0.16.3 -> 111.0.2+up0.16.3\nhelm.sh/images: added fleet-agent, fleet\n", stdout)
	require.Equal(t, oci.ChartRepo{Host: "stgregistry.suse.com", Repository: "rancher/catalog/fleet"}, fake.repo)
	require.Equal(t, oci.FetchOptions{PlainHTTP: false}, fake.opts)
}

func TestStampReadsTheRegistryGiven(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)
	fake := &fakeLister{}

	stdout, _, err := runStampWith(t, fake, dir, "--registry", "127.0.0.1:5052/e2e", "--plain-http")

	require.NoError(t, err)
	require.Equal(t, "version: 0.16.3 -> 111.0.0+up0.16.3\nhelm.sh/images: added fleet-agent, fleet\n", stdout)
	require.Equal(t, oci.ChartRepo{Host: "127.0.0.1:5052", Repository: "e2e/fleet"}, fake.repo)
	require.Equal(t, oci.FetchOptions{PlainHTTP: true}, fake.opts)
}

func TestStampStartsAChartThatWasNeverPushed(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, "")
	fake := &fakeLister{err: errors.New("not there: " + oci.ErrRepositoryNotFound.Error())}
	fake.err = errors.Join(fake.err, oci.ErrRepositoryNotFound)

	stdout, _, err := runStampWith(t, fake, dir)

	require.NoError(t, err)
	require.Equal(t, "version: 0.16.3 -> 111.0.0+up0.16.3\n", stdout)
}

func TestStampDoesNotAskTheRegistryForAGivenVersion(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, "")
	fake := &fakeLister{err: errors.New("the registry must not be asked")}

	_, _, err := runStampWith(t, fake, dir, "--version", "111.0.9+up0.16.3")

	require.NoError(t, err)
	require.False(t, fake.called)
}

func TestStampCannotWorkOutTheVersion(t *testing.T) {
	tests := []struct {
		name    string
		fake    *fakeLister
		wantErr string
	}{
		{
			name:    "the registry fails",
			fake:    &fakeLister{err: errors.New("the registry asks for authentication")},
			wantErr: "the registry asks for authentication",
		},
		{
			name:    "a tag that is not a version",
			fake:    &fakeLister{tags: []string{"latest"}},
			wantErr: `version "latest" must look like`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeChartDir(t, stampChartYAML, stampValuesYAML)

			stdout, _, err := runStampWith(t, tt.fake, dir)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			onDisk, readErr := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
			require.NoError(t, readErr)
			require.Equal(t, stampChartYAML, string(onDisk))
		})
	}
}

func TestStampRefusesAnEmptyVersion(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)
	fake := &fakeLister{tags: []string{"111.0.0_up0.16.1"}}

	stdout, _, err := runStampWith(t, fake, dir, "--version", "")

	require.ErrorContains(t, err, "--version is empty")
	require.Empty(t, stdout)
	require.False(t, fake.called, "an empty version that was passed must not turn into a calculation")
}

func TestStampDryRun(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)

	stdout, stderr, err := runStamp(t, dir, "--version", "111.0.0+up0.16.3", "--dry-run")

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "--- Chart.yaml\n+++ Chart.yaml\n"+
		"@@ -1,7 +1,12 @@\n"+
		" apiVersion: v2\n"+
		" name: fleet\n"+
		"-version: 0.16.3\n"+
		"+version: 111.0.0+up0.16.3\n"+
		" appVersion: 0.16.3\n"+
		" annotations:\n"+
		"   catalog.cattle.io/certified: rancher\n"+
		"   catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'\n"+
		"+  helm.sh/images: |\n"+
		"+    - name: fleet-agent\n"+
		"+      image: rancher/fleet-agent:v0.16.3\n"+
		"+    - name: fleet\n"+
		"+      image: rancher/fleet:v0.16.3\n", stdout)

	onDisk, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.Equal(t, stampChartYAML, string(onDisk), "a dry run writes nothing")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2, "and leaves no temporary file")
}

func TestStampDryRunWhenNothingChanges(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)
	_, _, err := runStamp(t, dir, "--version", "111.0.0+up0.16.3")
	require.NoError(t, err)

	stdout, _, err := runStamp(t, dir, "--version", "111.0.0+up0.16.3", "--dry-run")

	require.NoError(t, err)
	require.Equal(t, "nothing to change\n", stdout)
}

func TestStampDryRunStillChecksEverything(t *testing.T) {
	t.Run("a version that breaks the rules", func(t *testing.T) {
		dir := writeChartDir(t, stampChartYAML, stampValuesYAML)

		stdout, _, err := runStamp(t, dir, "--version", "111.0.0", "--dry-run")

		require.ErrorContains(t, err, `version "111.0.0" must look like 1XY.Z.W+upA.B.C`)
		require.Empty(t, stdout)
	})

	t.Run("an image with the tag latest", func(t *testing.T) {
		dir := writeChartDir(t, stampChartYAML, "image:\n  repository: rancher/fleet\n  tag: latest\n")

		stdout, _, err := runStamp(t, dir, "--version", "111.0.0+up0.16.3", "--dry-run")

		require.ErrorContains(t, err, `values.yaml image: "tag" must not be "latest"`)
		require.Empty(t, stdout)
	})
}

func TestStampDryRunWorksOutTheVersion(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, "")
	fake := &fakeLister{tags: []string{"111.0.4_up0.16.2"}}

	stdout, _, err := runStampWith(t, fake, dir, "--dry-run")

	require.NoError(t, err)
	require.Contains(t, stdout, "+version: 111.0.5+up0.16.3\n")
	require.True(t, fake.called, "the registry is read to work the version out")
	onDisk, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.Equal(t, stampChartYAML, string(onDisk))
}
