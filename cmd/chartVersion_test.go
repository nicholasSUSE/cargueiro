package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// repoLister answers with the tags of each repository, such as
// rancher/catalog/fleet, says that a repository it does not have is not found,
// and records the repositories it was asked about.
type repoLister struct {
	tags  map[string][]string
	asked []string
}

func (r *repoLister) list(_ context.Context, repo oci.ChartRepo, _ oci.FetchOptions) ([]string, error) {
	r.asked = append(r.asked, repo.Repository)
	tags, ok := r.tags[repo.Repository]
	if !ok {
		return nil, fmt.Errorf("%s was not found in the registry: %w", repo, oci.ErrRepositoryNotFound)
	}
	return tags, nil
}

// runChartVersionFamily executes chart-version with the fleet family of three
// charts in the rules.
func runChartVersionFamily(t *testing.T, registry *repoLister, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(familyRules()),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{listTags: registry.list})
	require.NoError(t, err)
	root.SetArgs(append([]string{"chart-version"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestChartVersionFamilyPrintsTheVersionOfTheMainChart(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	same := []string{"111.0.0_up0.16.1", "111.0.1_up0.16.2"}
	registry := &repoLister{tags: map[string][]string{
		"rancher/catalog/fleet":       same,
		"rancher/catalog/fleet-agent": same,
		"rancher/catalog/fleet-crd":   same,
	}}

	stdout, stderr, err := runChartVersionFamily(t, registry, "--family", "fleet", crd, agent, fleet)

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "111.0.2+up0.16.3\n", stdout, "one version for the family, whatever the order of the folders")
	require.Equal(t, []string{"rancher/catalog/fleet", "rancher/catalog/fleet-crd", "rancher/catalog/fleet-agent"}, registry.asked,
		"the main chart first, then the order of the family, one read for each")
	require.Equal(t, stampChartYAML, readChartFile(t, fleet), "nothing is written")
}

func TestChartVersionFamilyStartsAFamilyThatWasNeverPushed(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)

	stdout, _, err := runChartVersionFamily(t, &repoLister{}, "--family", "fleet", fleet, agent, crd)

	require.NoError(t, err)
	require.Equal(t, "111.0.0+up0.16.3\n", stdout)
}

func TestChartVersionFamilyRefusesWhenTheLatestVersionsDiffer(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	registry := &repoLister{tags: map[string][]string{
		"rancher/catalog/fleet":       {"111.0.0_up0.16.1", "111.0.1_up0.16.2"},
		"rancher/catalog/fleet-agent": {"111.0.0_up0.16.1", "111.0.1_up0.16.2"},
		"rancher/catalog/fleet-crd":   {"111.0.0_up0.16.1"},
	}}

	stdout, _, err := runChartVersionFamily(t, registry, "--family", "fleet", fleet, agent, crd)

	require.ErrorContains(t, err, `the charts of the family "fleet" are not at the same released version: `+
		"fleet 111.0.1+up0.16.2, fleet-crd 111.0.0+up0.16.1, fleet-agent 111.0.1+up0.16.2")
	require.Empty(t, stdout)
}

func TestChartVersionFamilyRefusesAndReadsNoRegistry(t *testing.T) {
	fleet, agent, crd := stampFamilyDirs(t)
	longhorn := writeChartDir(t, familyChartYAML("longhorn", "0.16.3"), "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "a chart is missing",
			args:    []string{"--family", "fleet", fleet, crd},
			wantErr: `the family "fleet" is missing fleet-agent`,
		},
		{
			name:    "a chart is given twice",
			args:    []string{"--family", "fleet", fleet, fleet, agent, crd},
			wantErr: `chart "fleet" is given twice`,
		},
		{
			name:    "a chart of another family",
			args:    []string{"--family", "fleet", fleet, agent, crd, longhorn},
			wantErr: `chart "longhorn" is not in the family "fleet"`,
		},
		{
			name:    "a family that is not listed",
			args:    []string{"--family", "nope", fleet},
			wantErr: `family "nope" is not listed in the rules file`,
		},
		{
			name:    "no folders",
			args:    []string{"--family", "fleet"},
			wantErr: "requires at least 1 arg(s)",
		},
		{
			name:    "a folder that does not exist",
			args:    []string{"--family", "fleet", fleet, agent, filepath.Join(t.TempDir(), "nope")},
			wantErr: "Chart.yaml not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &repoLister{}

			stdout, _, err := runChartVersionFamily(t, registry, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.Empty(t, registry.asked, "it is decided from the rules and the folders, before the network")
		})
	}
}

func TestChartVersionFamilyReportsTheMainChartWhenItCannotHaveAVersion(t *testing.T) {
	_, agent, crd := stampFamilyDirs(t)
	noAppVersion := writeChartDir(t, "apiVersion: v2\nname: fleet\nversion: 0.16.3\nannotations:\n  catalog.cattle.io/certified: rancher\n  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'\n", "")

	stdout, _, err := runChartVersionFamily(t, &repoLister{}, "--family", "fleet", noAppVersion, agent, crd)

	require.ErrorContains(t, err, "fleet: the Chart.yaml has no appVersion")
	require.Empty(t, stdout)
}
