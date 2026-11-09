package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeArchive writes a chart archive with the files that are not empty, and
// returns its path.
func writeArchive(t *testing.T, chartYAML, valuesYAML string) string {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{"Chart.yaml": chartYAML, "values.yaml": valuesYAML} {
		if body == "" {
			continue
		}
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     "chart/" + name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(body)),
		}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	path := filepath.Join(t.TempDir(), "chart.tgz")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

// runCheckArchive executes check archive with the rules in checkRules.
func runCheckArchive(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{})
	require.NoError(t, err)
	root.SetArgs(append([]string{"check", "archive"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

// TestCheckArchiveReportsLikeCheckChart makes sure that the same files give
// the same report from an archive as from a directory.
func TestCheckArchiveReportsLikeCheckChart(t *testing.T) {
	tests := []struct {
		name       string
		chartYAML  string
		valuesYAML string
		wantErr    string
	}{
		{
			name:       "a valid chart",
			chartYAML:  validChartYAML,
			valuesYAML: "a: 1\n",
		},
		{
			name:       "a chart that breaks the rules",
			chartYAML:  noCertifiedChartYAML,
			valuesYAML: imagesValuesYAML,
			wantErr:    "chart is not valid",
		},
	}
	for _, tt := range tests {
		for _, format := range []string{"text", "json"} {
			t.Run(tt.name+", "+format, func(t *testing.T) {
				dir := writeChartDir(t, tt.chartYAML, tt.valuesYAML)
				archive := writeArchive(t, tt.chartYAML, tt.valuesYAML)

				wantOut, _, wantErr := runCheck(t, dir, "--output", format)
				gotOut, _, gotErr := runCheckArchive(t, archive, "--output", format)

				require.NotEmpty(t, gotOut)
				require.Equal(t, wantOut, gotOut)
				if tt.wantErr == "" {
					require.NoError(t, wantErr)
					require.NoError(t, gotErr)
					return
				}
				require.ErrorContains(t, gotErr, tt.wantErr)
			})
		}
	}
}

func TestCheckArchiveNoErrorHIP15(t *testing.T) {
	archive := writeArchive(t, validChartYAML, imagesValuesYAML)

	_, _, err := runCheckArchive(t, archive)
	require.ErrorContains(t, err, "chart is not valid")

	stdout, stderr, err := runCheckArchive(t, archive, "--no-error-hip-15")
	require.NoError(t, err)
	require.Contains(t, stdout, "hip15-annotations: invalid\n")
	require.Equal(t, "chart is not valid (HIP-15 errors ignored by --no-error-hip-15)\n", stderr)
}

func TestCheckArchiveFailsWithoutPrinting(t *testing.T) {
	notAnArchive := filepath.Join(t.TempDir(), "not.tgz")
	require.NoError(t, os.WriteFile(notAnArchive, []byte("this is not an archive"), 0o600))

	tests := []struct {
		name    string
		args    func(t *testing.T) []string
		wantErr string
	}{
		{
			name: "the file does not exist",
			args: func(t *testing.T) []string {
				return []string{filepath.Join(t.TempDir(), "nope.tgz")}
			},
			wantErr: "nope.tgz not found",
		},
		{
			name: "it is a directory",
			args: func(t *testing.T) []string {
				return []string{t.TempDir()}
			},
			wantErr: "the archive is not gzip",
		},
		{
			name: "it is not an archive",
			args: func(*testing.T) []string {
				return []string{notAnArchive}
			},
			wantErr: "the archive is not gzip",
		},
		{
			name: "no values.yaml",
			args: func(t *testing.T) []string {
				return []string{writeArchive(t, validChartYAML, "")}
			},
			wantErr: "the archive has no values.yaml",
		},
		{
			name: "the chart is not listed in the rules",
			args: func(t *testing.T) []string {
				return []string{writeArchive(t, unlistedChartYAML, "a: 1\n")}
			},
			wantErr: `chart "epinio" is not listed in the rules file`,
		},
		{
			name: "an unknown output format",
			args: func(t *testing.T) []string {
				return []string{writeArchive(t, validChartYAML, "a: 1\n"), "-o", "yaml"}
			},
			wantErr: `unknown output format "yaml"`,
		},
		{
			name: "no archive",
			args: func(*testing.T) []string {
				return nil
			},
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name: "two archives",
			args: func(t *testing.T) []string {
				return []string{writeArchive(t, validChartYAML, "a: 1\n"), notAnArchive}
			},
			wantErr: "accepts 1 arg(s), received 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runCheckArchive(t, tt.args(t)...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
		})
	}
}
