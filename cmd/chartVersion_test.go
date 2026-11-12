package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/oci"
)

// runChartVersion executes chart-version with the rules in checkRules, the
// registry and the environment given.
func runChartVersion(t *testing.T, fake *fakeLister, env map[string]string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(name string) string { return env[name] },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{listTags: fake.list})
	require.NoError(t, err)
	root.SetArgs(append([]string{"chart-version"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestChartVersionPrintsTheVersion(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)
	fake := &fakeLister{tags: []string{"111.0.0_up0.16.1", "111.0.1_up0.16.2"}}

	stdout, stderr, err := runChartVersion(t, fake, nil, dir)

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "111.0.2+up0.16.3\n", stdout, "only the version")
	require.Equal(t, oci.ChartRepo{Host: "stgregistry.suse.com", Repository: "rancher/catalog/fleet"}, fake.repo)
	require.Equal(t, oci.FetchOptions{PlainHTTP: false}, fake.opts)
	require.Equal(t, stampChartYAML, readChartFile(t, dir), "nothing is written")
}

func TestChartVersionReadsTheRegistryGiven(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)
	fake := &fakeLister{}

	stdout, _, err := runChartVersion(t, fake, nil, dir, "--registry", "127.0.0.1:5052/e2e", "--plain-http")

	require.NoError(t, err)
	require.Equal(t, "111.0.0+up0.16.3\n", stdout)
	require.Equal(t, oci.ChartRepo{Host: "127.0.0.1:5052", Repository: "e2e/fleet"}, fake.repo)
	require.Equal(t, oci.FetchOptions{PlainHTTP: true}, fake.opts)
}

func TestChartVersionReadsWithTheCredentialsGiven(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, stampValuesYAML)
	fake := &fakeLister{}
	env := map[string]string{"CARGUEIRO_USERNAME": "reader", "CARGUEIRO_PASSWORD": "secret"}

	_, _, err := runChartVersion(t, fake, env, dir)

	require.NoError(t, err)
	require.Equal(t, oci.FetchOptions{Username: "reader", Password: "secret"}, fake.opts)
}

func TestChartVersionStartsAChartThatWasNeverPushed(t *testing.T) {
	dir := writeChartDir(t, stampChartYAML, "")
	fake := &fakeLister{err: errors.Join(errors.New("not there"), oci.ErrRepositoryNotFound)}

	stdout, _, err := runChartVersion(t, fake, nil, dir)

	require.NoError(t, err)
	require.Equal(t, "111.0.0+up0.16.3\n", stdout)
}

func TestChartVersionCannotWorkOutTheVersion(t *testing.T) {
	tests := []struct {
		name      string
		chartYAML string
		fake      *fakeLister
		wantErr   string
	}{
		{
			name:      "the registry fails",
			chartYAML: stampChartYAML,
			fake:      &fakeLister{err: errors.New("the registry asks for authentication")},
			wantErr:   "the registry asks for authentication",
		},
		{
			name:      "a tag that is not a version",
			chartYAML: stampChartYAML,
			fake:      &fakeLister{tags: []string{"latest"}},
			wantErr:   `version "latest" must look like`,
		},
		{
			name:      "no appVersion",
			chartYAML: "apiVersion: v2\nname: fleet\nversion: 0.16.3\nannotations:\n  catalog.cattle.io/certified: rancher\n  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'\n",
			fake:      &fakeLister{},
			wantErr:   "the Chart.yaml has no appVersion",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeChartDir(t, tt.chartYAML, stampValuesYAML)

			stdout, _, err := runChartVersion(t, tt.fake, nil, dir)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.Equal(t, tt.chartYAML, readChartFile(t, dir))
		})
	}
}

func TestChartVersionNeedsADirectory(t *testing.T) {
	t.Run("no directory", func(t *testing.T) {
		_, _, err := runChartVersion(t, &fakeLister{}, nil)

		require.ErrorContains(t, err, "accepts 1 arg(s), received 0")
	})

	t.Run("two directories", func(t *testing.T) {
		_, _, err := runChartVersion(t, &fakeLister{}, nil, t.TempDir(), t.TempDir())

		require.ErrorContains(t, err, "accepts 1 arg(s), received 2")
	})

	t.Run("a directory that does not exist", func(t *testing.T) {
		fake := &fakeLister{}

		_, _, err := runChartVersion(t, fake, nil, filepath.Join(t.TempDir(), "nope"))

		require.ErrorContains(t, err, "Chart.yaml not found")
		require.False(t, fake.called)
	})
}
