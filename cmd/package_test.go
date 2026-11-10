package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/chart"
)

// stampedChartYAML is stampChartYAML after stamp: the final version, and the
// helm.sh/images annotation for the images of stampValuesYAML' first block.
const stampedChartYAML = `apiVersion: v2
name: fleet
version: 111.0.0+up0.16.3
appVersion: 0.16.3
annotations:
  catalog.cattle.io/certified: rancher
  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'
  helm.sh/images: |
    - name: fleet
      image: rancher/fleet:v0.16.3
`

const stampedValuesYAML = `image:
  repository: rancher/fleet
  tag: v0.16.3
`

// countingPackager is the real packager, and counts its calls.
type countingPackager struct {
	calls int
	// pack replaces the real packager when it is set.
	pack packageFunc
}

func (c *countingPackager) pkg(ctx context.Context, src, dest string) (string, error) {
	c.calls++
	if c.pack != nil {
		return c.pack(ctx, src, dest)
	}
	return chart.Package(ctx, src, dest)
}

// writeFiles adds files to a chart directory, named by their path in it.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, body := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
}

// runPackage executes package with the rules in checkRules.
func runPackage(t *testing.T, packager *countingPackager, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{pkg: packager.pkg})
	require.NoError(t, err)
	root.SetArgs(append([]string{"package"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestPackage(t *testing.T) {
	dir := writeChartDir(t, stampedChartYAML, stampedValuesYAML)
	writeFiles(t, dir, map[string]string{"templates/deployment.yaml": "kind: Deployment\n"})
	out := filepath.Join(t.TempDir(), "out")
	packager := &countingPackager{}

	stdout, stderr, err := runPackage(t, packager, dir, "-o", out)

	require.NoError(t, err)
	require.Empty(t, stderr, "nothing but the path is printed")
	archive := filepath.Join(out, "fleet-111.0.0+up0.16.3.tgz")
	require.Equal(t, archive+"\n", stdout)
	require.FileExists(t, archive)
	require.Equal(t, 1, packager.calls)
}

func TestPackageLogsWhatItVerifies(t *testing.T) {
	dir := writeChartDir(t, stampedChartYAML, stampedValuesYAML)
	writeFiles(t, dir, map[string]string{
		"templates/deployment.yaml": "kind: Deployment\n",
		"templates/old.bak":         "old\n",
		".helmignore":               "*.bak\n",
	})

	stdout, stderr, err := runPackage(t, &countingPackager{}, "--debug", dir, "-o", t.TempDir())

	require.NoError(t, err)
	require.Contains(t, stdout, "fleet-111.0.0+up0.16.3.tgz")
	for _, line := range []string{
		"verifying the chart",
		"packaging chart",
		"verifying the archive",
		"verifying digests",
		"files of the directory that are not in the archive",
		"templates/old.bak",
	} {
		require.Contains(t, stderr, line)
	}
}

func TestPackageRefusesAChartThatIsNotValid(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML) // the chart before stamp
	out := filepath.Join(t.TempDir(), "out")
	packager := &countingPackager{}

	stdout, stderr, err := runPackage(t, packager, dir, "-o", out)

	require.EqualError(t, err, "the chart is not valid, so it was not packaged")
	require.Empty(t, stdout)
	require.Contains(t, stderr, "version: invalid")
	require.Contains(t, stderr, "hip15-annotations: invalid")
	require.Zero(t, packager.calls, "nothing is packaged")
	require.NoDirExists(t, out)
}

func TestPackageDoesNotIgnoreHIP15(t *testing.T) {
	// Only the helm.sh/images annotation is missing.
	chartYAML := stampedChartYAML[:len(stampedChartYAML)-len("  helm.sh/images: |\n    - name: fleet\n      image: rancher/fleet:v0.16.3\n")]
	dir := writeChartDir(t, chartYAML, stampedValuesYAML)
	packager := &countingPackager{}

	_, stderr, err := runPackage(t, packager, dir, "-o", t.TempDir())

	require.ErrorContains(t, err, "the chart is not valid")
	require.Contains(t, stderr, "hip15-annotations: invalid")
	require.Zero(t, packager.calls)
}

func TestPackageFailsWithoutPackaging(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T) string
		wantErr string
	}{
		{
			name: "a chart that is not in the rules file",
			prepare: func(t *testing.T) string {
				chartYAML := "apiVersion: v2\nname: epinio\nversion: 111.0.0+up1.0.0\n"
				return writeChartDir(t, chartYAML, stampedValuesYAML)
			},
			wantErr: `chart "epinio" is not listed in the rules file`,
		},
		{
			name:    "a directory with no Chart.yaml",
			prepare: func(t *testing.T) string { return t.TempDir() },
			wantErr: "Chart.yaml not found",
		},
		{
			name:    "a directory that does not exist",
			prepare: func(t *testing.T) string { return filepath.Join(t.TempDir(), "nope") },
			wantErr: "Chart.yaml not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packager := &countingPackager{}

			stdout, _, err := runPackage(t, packager, tt.prepare(t), "-o", t.TempDir())

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.Zero(t, packager.calls)
		})
	}
}

func TestPackageReturnsTheErrorOfThePackager(t *testing.T) {
	errBoom := errors.New("boom")
	dir := writeChartDir(t, stampedChartYAML, stampedValuesYAML)
	packager := &countingPackager{pack: func(context.Context, string, string) (string, error) { return "", errBoom }}

	stdout, _, err := runPackage(t, packager, dir)

	require.ErrorIs(t, err, errBoom)
	require.Empty(t, stdout)
}

func TestPackageRemovesAnArchiveThatIsNotValid(t *testing.T) {
	dir := writeChartDir(t, stampedChartYAML, stampedValuesYAML)
	before := writeChartDir(t, stampChartYAML, stampValuesYAML) // not stamped
	out := t.TempDir()
	var archive string
	packager := &countingPackager{pack: func(ctx context.Context, _, dest string) (string, error) {
		var err error
		archive, err = chart.Package(ctx, before, dest)
		return archive, err
	}}

	stdout, stderr, err := runPackage(t, packager, dir, "-o", out)

	require.EqualError(t, err, "the archive is not valid, so it was removed")
	require.Empty(t, stdout)
	require.Contains(t, stderr, "version: invalid")
	require.NotEmpty(t, archive)
	require.NoFileExists(t, archive)
}

func TestPackageRemovesAnArchiveThatDoesNotHaveTheFilesOfTheDirectory(t *testing.T) {
	dir := writeChartDir(t, stampedChartYAML, stampedValuesYAML)
	writeFiles(t, dir, map[string]string{"templates/deployment.yaml": "kind: Deployment\n"})

	// The archive is of a valid chart, but not of this one.
	other := writeChartDir(t, stampedChartYAML, stampedValuesYAML)
	writeFiles(t, other, map[string]string{
		"templates/deployment.yaml": "kind: Deployment\nreplicas: 99\n",
		"templates/extra.yaml":      "kind: Extra\n",
	})
	var archive string
	packager := &countingPackager{pack: func(ctx context.Context, _, dest string) (string, error) {
		var err error
		archive, err = chart.Package(ctx, other, dest)
		return archive, err
	}}

	stdout, _, err := runPackage(t, packager, dir, "-o", t.TempDir())

	require.ErrorContains(t, err, "the archive does not have the files of the directory, so it was removed")
	require.ErrorContains(t, err, "2 files differ between the directory and the archive:")
	require.ErrorContains(t, err, "templates/deployment.yaml: the directory has sha256:")
	require.ErrorContains(t, err, "templates/extra.yaml: only in the archive")
	require.Empty(t, stdout)
	require.NoFileExists(t, archive)
}

func TestPackageNeedsOneChart(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no chart given",
			args:    nil,
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name:    "two charts",
			args:    []string{"charts/a", "charts/b"},
			wantErr: "accepts 1 arg(s), received 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := runPackage(t, &countingPackager{}, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestPackageWithTheRealCommand makes sure the command as the binary builds it
// packages, and uses the default output directory.
func TestPackageWithTheRealCommand(t *testing.T) {
	dir := writeChartDir(t, stampedChartYAML, stampedValuesYAML)
	t.Chdir(t.TempDir())

	var out, errOut bytes.Buffer
	root, err := NewRootCmd(&out, &errOut, []byte(checkRules))
	require.NoError(t, err)
	root.SetArgs([]string{"package", dir})

	require.NoError(t, root.Execute())
	require.Equal(t, "/", out.String()[:1], "the path of the archive is absolute")
	require.FileExists(t, filepath.Join("assets", "fleet-111.0.0+up0.16.3.tgz"))
	require.Empty(t, errOut.String())
}
