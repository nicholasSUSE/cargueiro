package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const checkRules = `
kinds:
  main:
    required: [certified]
families:
  - family: fleet
    charts:
      - { name: fleet, kind: main, edition: rancher-charts }
`

const validChartYAML = `
name: fleet
version: 110.0.3
appVersion: 0.16.3
icon: https://example.com/icon.png
description: Fleet
annotations:
  catalog.cattle.io/certified: rancher
  catalog.cattle.io/kube-version: ">= 1.30.0-0 < 1.35.0-0"
`

const unlistedChartYAML = `
name: epinio
version: 1.0.0
`

const noCertifiedChartYAML = `
name: fleet
version: 110.0.3
`

const imagesValuesYAML = `
image:
  repository: rancher/fleet
  tag: v0.16.3
`

// writeChartDir writes a chart directory and returns it. An empty content
// skips the file.
func writeChartDir(t *testing.T, chartYAML, valuesYAML string) string {
	t.Helper()
	dir := t.TempDir()
	if chartYAML != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYAML), 0o600))
	}
	if valuesYAML != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(valuesYAML), 0o600))
	}
	return dir
}

// runCheck executes check chart with the rules in checkRules.
func runCheck(t *testing.T, args ...string) (stdout, stderr string, err error) {
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
	root.SetArgs(append([]string{"check", "chart"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestCheckChartValid(t *testing.T) {
	dir := writeChartDir(t, validChartYAML, "a: 1\n")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "text is the default",
			args: []string{dir},
			want: `valid: true
annotations: valid
hip15-annotations: valid
chart: fleet
kind: main
family: fleet
metadata:
  name: fleet
  version: 110.0.3
  appVersion: 0.16.3
  icon: https://example.com/icon.png
  description: Fleet
  kubeVersion: >= 1.30.0-0 < 1.35.0-0
`,
		},
		{
			name: "json",
			args: []string{dir, "-o", "json"},
			want: `{
  "valid": true,
  "annotations": {
    "valid": true,
    "errors": []
  },
  "hip15-annotations": {
    "valid": true,
    "errors": []
  },
  "chart": "fleet",
  "kind": "main",
  "family": "fleet",
  "metadata": {
    "name": "fleet",
    "version": "110.0.3",
    "appVersion": "0.16.3",
    "icon": "https://example.com/icon.png",
    "description": "Fleet",
    "kubeVersion": ">= 1.30.0-0 < 1.35.0-0"
  }
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runCheck(t, tt.args...)
			require.NoError(t, err)
			require.Equal(t, tt.want, stdout)
			require.Empty(t, stderr)
		})
	}
}

func TestCheckChartPrintsViolationsAndFails(t *testing.T) {
	dir := writeChartDir(t, noCertifiedChartYAML, imagesValuesYAML)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "text",
			args: []string{dir},
			want: `valid: false
annotations: invalid
  - missing annotation catalog.cattle.io/certified
hip15-annotations: invalid
  - values.yaml image: image "rancher/fleet:v0.16.3" is not listed in helm.sh/images
chart: fleet
kind: main
family: fleet
metadata:
  name: fleet
  version: 110.0.3
  appVersion:
  icon:
  description:
  kubeVersion:
`,
		},
		{
			name: "json",
			args: []string{dir, "--output", "json"},
			want: `{
  "valid": false,
  "annotations": {
    "valid": false,
    "errors": [
      "missing annotation catalog.cattle.io/certified"
    ]
  },
  "hip15-annotations": {
    "valid": false,
    "errors": [
      "values.yaml image: image \"rancher/fleet:v0.16.3\" is not listed in helm.sh/images"
    ]
  },
  "chart": "fleet",
  "kind": "main",
  "family": "fleet",
  "metadata": {
    "name": "fleet",
    "version": "110.0.3",
    "appVersion": "",
    "icon": "",
    "description": "",
    "kubeVersion": ""
  }
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runCheck(t, tt.args...)
			require.ErrorContains(t, err, "chart is not valid")
			require.Equal(t, tt.want, stdout)
		})
	}
}

func TestCheckChartFailsWithoutPrinting(t *testing.T) {
	tests := []struct {
		name    string
		args    func(t *testing.T) []string
		wantErr string
	}{
		{
			name: "directory does not exist",
			args: func(t *testing.T) []string {
				return []string{filepath.Join(t.TempDir(), "nope")}
			},
			wantErr: "Chart.yaml not found",
		},
		{
			name: "no values.yaml",
			args: func(t *testing.T) []string {
				return []string{writeChartDir(t, validChartYAML, "")}
			},
			wantErr: "values.yaml not found",
		},
		{
			name: "Chart.yaml is not yaml",
			args: func(t *testing.T) []string {
				return []string{writeChartDir(t, "name: [\n", "a: 1\n")}
			},
			wantErr: "parse",
		},
		{
			name: "chart is not listed in the rules",
			args: func(t *testing.T) []string {
				return []string{writeChartDir(t, unlistedChartYAML, "a: 1\n")}
			},
			wantErr: `chart "epinio" is not listed in the rules file`,
		},
		{
			name: "unknown output format",
			args: func(t *testing.T) []string {
				return []string{writeChartDir(t, validChartYAML, "a: 1\n"), "-o", "yaml"}
			},
			wantErr: `unknown output format "yaml"`,
		},
		{
			name: "no chart directory",
			args: func(*testing.T) []string {
				return nil
			},
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name: "two chart directories",
			args: func(t *testing.T) []string {
				return []string{t.TempDir(), t.TempDir()}
			},
			wantErr: "accepts 1 arg(s), received 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runCheck(t, tt.args(t)...)
			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
		})
	}
}

// TestCheckChartNoErrorHIP15 checks that the flag changes the exit code and
// nothing in what is printed on stdout.
func TestCheckChartNoErrorHIP15(t *testing.T) {
	const ignored = "chart is not valid (HIP-15 errors ignored by --no-error-hip-15)\n"

	onlyHIP15 := writeChartDir(t, validChartYAML, imagesValuesYAML)
	annotationsToo := writeChartDir(t, noCertifiedChartYAML, imagesValuesYAML)
	valid := writeChartDir(t, validChartYAML, "a: 1\n")

	for _, format := range []string{"text", "json"} {
		t.Run("only HIP-15 errors, "+format, func(t *testing.T) {
			withoutFlag, _, err := runCheck(t, onlyHIP15, "--output", format)
			require.ErrorContains(t, err, "chart is not valid")

			stdout, stderr, err := runCheck(t, onlyHIP15, "--output", format, "--no-error-hip-15")
			require.NoError(t, err)
			require.Equal(t, withoutFlag, stdout)
			require.Equal(t, ignored, stderr)
		})

		t.Run("annotation errors still fail, "+format, func(t *testing.T) {
			withoutFlag, _, err := runCheck(t, annotationsToo, "--output", format)
			require.ErrorContains(t, err, "chart is not valid")

			stdout, stderr, err := runCheck(t, annotationsToo, "--output", format, "--no-error-hip-15")
			require.ErrorContains(t, err, "chart is not valid")
			require.Equal(t, withoutFlag, stdout)
			require.Empty(t, stderr)
		})

		t.Run("valid chart prints no message, "+format, func(t *testing.T) {
			stdout, stderr, err := runCheck(t, valid, "--output", format, "--no-error-hip-15")
			require.NoError(t, err)
			require.NotEmpty(t, stdout)
			require.Empty(t, stderr)
		})
	}
}
