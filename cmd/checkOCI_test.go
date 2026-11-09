package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/check"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// fetchCall is one call of the fake fetcher.
type fetchCall struct {
	ref  oci.ChartRef
	opts oci.FetchOptions
}

// fakeFetcher returns files or err, and records its calls.
type fakeFetcher struct {
	files *chart.Files
	err   error
	calls []fetchCall
}

func (f *fakeFetcher) fetch(_ context.Context, ref oci.ChartRef, opts oci.FetchOptions) (*chart.Files, error) {
	f.calls = append(f.calls, fetchCall{ref: ref, opts: opts})
	return f.files, f.err
}

// runCheckOCI executes check oci with the rules in checkRules.
func runCheckOCI(t *testing.T, fake *fakeFetcher, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{fetchChart: fake.fetch})
	require.NoError(t, err)
	root.SetArgs(append([]string{"check", "oci"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func validFiles() *chart.Files {
	return &chart.Files{
		Metadata: chart.Metadata{Name: "fleet", Version: "111.0.3+up0.16.3"},
		Annotations: map[string]string{
			"catalog.cattle.io/certified":    "rancher",
			"catalog.cattle.io/kube-version": ">= 1.30.0-0 < 1.35.0-0",
		},
	}
}

func TestCheckOCIValid(t *testing.T) {
	fake := &fakeFetcher{files: validFiles()}

	t.Run("text", func(t *testing.T) {
		stdout, stderr, err := runCheckOCI(t, fake, "registry.example.com/rancher/fleet:111.0.3+up0.16.3")
		require.NoError(t, err)
		require.Empty(t, stderr)
		require.Equal(t, `valid: true
annotations: valid
hip15-annotations: valid
version: valid
chart: fleet
kind: main
family: fleet
metadata:
  name: fleet
  version: 111.0.3+up0.16.3
  appVersion:
  icon:
  description:
  kubeVersion: >= 1.30.0-0 < 1.35.0-0
`, stdout)
	})

	t.Run("json", func(t *testing.T) {
		stdout, _, err := runCheckOCI(t, fake, "registry.example.com/rancher/fleet:111.0.3+up0.16.3", "-o", "json")
		require.NoError(t, err)

		var got check.Result
		require.NoError(t, json.Unmarshal([]byte(stdout), &got))
		require.True(t, got.Valid)
		require.Equal(t, "fleet", got.Chart)
		require.Equal(t, "main", got.Kind)
	})
}

// TestCheckOCIValidatesTheVersionOfTheReference makes sure the version checked
// is the one on the command line, read back from the tag, and not the one in
// the Chart.yaml of the fetched chart.
func TestCheckOCIValidatesTheVersionOfTheReference(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		wantValid bool
		wantError string
	}{
		{
			name:      "a tag as the registry stores it",
			reference: "registry.example.com/rancher/fleet:111.0.3_up0.16.3",
			wantValid: true,
		},
		{
			name:      "a version with a plus",
			reference: "registry.example.com/rancher/fleet:111.0.3+up0.16.3",
			wantValid: true,
		},
		{
			name:      "a tag that is not a Rancher version",
			reference: "registry.example.com/rancher/fleet:latest",
			wantError: `version "latest" must look like 1XY.Z.W+upA.B.C`,
		},
		{
			name:      "a tag without the +up part",
			reference: "registry.example.com/rancher/fleet:111.0.3",
			wantError: `version "111.0.3" must look like 1XY.Z.W+upA.B.C`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeFetcher{files: validFiles()} // its Chart.yaml version is valid

			stdout, _, err := runCheckOCI(t, fake, tt.reference, "-o", "json")

			var got check.Result
			require.NoError(t, json.Unmarshal([]byte(stdout), &got))
			require.Equal(t, tt.wantValid, got.Version.Valid)
			if tt.wantValid {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, "chart is not valid")
			require.Len(t, got.Version.Errors, 1)
			require.Contains(t, got.Version.Errors[0], tt.wantError)
		})
	}
}

func TestCheckOCIAsksForTheReference(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want fetchCall
	}{
		{
			name: "a reference",
			args: []string{"registry.example.com/rancher/fleet:111.0.3_up0.16.3"},
			want: fetchCall{
				ref:  oci.ChartRef{Host: "registry.example.com", Repository: "rancher/fleet", Tag: "111.0.3_up0.16.3"},
				opts: oci.FetchOptions{PlainHTTP: false},
			},
		},
		{
			name: "the scheme and a plus in the version",
			args: []string{"oci://registry.example.com/rancher/fleet:111.0.3+up0.16.3"},
			want: fetchCall{
				ref:  oci.ChartRef{Host: "registry.example.com", Repository: "rancher/fleet", Tag: "111.0.3_up0.16.3"},
				opts: oci.FetchOptions{PlainHTTP: false},
			},
		},
		{
			name: "plain HTTP",
			args: []string{"127.0.0.1:5050/rancher/fleet:111.0.3_up0.16.3", "--plain-http"},
			want: fetchCall{
				ref:  oci.ChartRef{Host: "127.0.0.1:5050", Repository: "rancher/fleet", Tag: "111.0.3_up0.16.3"},
				opts: oci.FetchOptions{PlainHTTP: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeFetcher{files: validFiles()}

			_, _, err := runCheckOCI(t, fake, tt.args...)

			require.NoError(t, err)
			require.Equal(t, []fetchCall{tt.want}, fake.calls)
		})
	}
}

func TestCheckOCIPrintsViolationsAndFails(t *testing.T) {
	files := validFiles()
	files.Annotations = map[string]string{}
	fake := &fakeFetcher{files: files}

	stdout, _, err := runCheckOCI(t, fake, "registry.example.com/rancher/fleet:1.0.0")

	require.ErrorContains(t, err, "chart is not valid")
	require.Contains(t, stdout, "valid: false\nannotations: invalid\n  - missing annotation catalog.cattle.io/certified\n")
}

func TestCheckOCINoErrorHIP15(t *testing.T) {
	files := validFiles()
	files.Values = map[string]any{
		"image": map[string]any{"repository": "rancher/fleet", "tag": "v1"},
	}
	fake := &fakeFetcher{files: files}
	const ref = "registry.example.com/rancher/fleet:111.0.3_up0.16.3"

	_, _, err := runCheckOCI(t, fake, ref)
	require.ErrorContains(t, err, "chart is not valid")

	stdout, stderr, err := runCheckOCI(t, fake, ref, "--no-error-hip-15")
	require.NoError(t, err)
	require.Contains(t, stdout, "hip15-annotations: invalid\n")
	require.Equal(t, "chart is not valid (HIP-15 errors ignored by --no-error-hip-15)\n", stderr)
}

func TestCheckOCIFailsWithoutPrinting(t *testing.T) {
	notListed := &chart.Files{Metadata: chart.Metadata{Name: "epinio"}}

	tests := []struct {
		name      string
		fake      *fakeFetcher
		args      []string
		wantErr   string
		wantCalls int
	}{
		{
			name:      "the fetch fails",
			fake:      &fakeFetcher{err: errors.New("registry.example.com/rancher/fleet:1.0.0 was not found in the registry")},
			args:      []string{"registry.example.com/rancher/fleet:1.0.0"},
			wantErr:   "was not found in the registry",
			wantCalls: 1,
		},
		{
			name:      "the chart is not listed in the rules",
			fake:      &fakeFetcher{files: notListed},
			args:      []string{"registry.example.com/rancher/epinio:1.0.0"},
			wantErr:   `chart "epinio" is not listed in the rules file`,
			wantCalls: 1,
		},
		{
			name:    "no tag",
			fake:    &fakeFetcher{files: validFiles()},
			args:    []string{"registry.example.com/rancher/fleet"},
			wantErr: "has no tag",
		},
		{
			name:    "a digest",
			fake:    &fakeFetcher{files: validFiles()},
			args:    []string{"registry.example.com/rancher/fleet@sha256:de5ef718a024560f857aad82cb95624067c13d0248455d498a25bea5cb9bc742"},
			wantErr: "references by digest",
		},
		{
			name:    "an unknown output format",
			fake:    &fakeFetcher{files: validFiles()},
			args:    []string{"registry.example.com/rancher/fleet:1.0.0", "-o", "yaml"},
			wantErr: `unknown output format "yaml"`,
		},
		{
			name:    "no reference",
			fake:    &fakeFetcher{files: validFiles()},
			args:    nil,
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name:    "two references",
			fake:    &fakeFetcher{files: validFiles()},
			args:    []string{"registry.example.com/a:1", "registry.example.com/b:1"},
			wantErr: "accepts 1 arg(s), received 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runCheckOCI(t, tt.fake, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.Len(t, tt.fake.calls, tt.wantCalls, "nothing is fetched for a request that cannot be valid")
		})
	}
}
