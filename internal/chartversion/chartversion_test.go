package chartversion

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
)

// loadRules parses the rules file of the repository, as the binary embeds it.
func loadRules(t *testing.T) *rules.Config {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "rules.yaml"))
	require.NoError(t, err)
	cfg, err := rules.Parse(data)
	require.NoError(t, err)
	return cfg
}

// copyChart copies Chart.yaml and values.yaml (if the fixture has one) of a
// chart kept in tests/data to a new directory, and returns it.
func copyChart(t *testing.T, dataDir, name string) string {
	t.Helper()

	src := filepath.Join("..", "..", "tests", "data", "charts", dataDir, "src", name)
	dst := t.TempDir()
	for _, file := range []string{"Chart.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join(src, file))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dst, file), data, 0o600))
	}
	return dst
}

// edit changes a file of the chart directory, which must contain old once.
func edit(t *testing.T, dir, file, old, replacement string) {
	t.Helper()

	path := filepath.Join(dir, file)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(data), old), "%s must contain %q once", file, old)
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), old, replacement, 1)), 0o600))
}

// next works out the version of the chart in dir.
func next(t *testing.T, cfg *rules.Config, dir string, opts Options) (string, []string, error) {
	t.Helper()

	files, err := chart.Load(dir)
	require.NoError(t, err)
	rule, err := cfg.Find(files.Metadata.Name)
	require.NoError(t, err)
	return Next(context.Background(), cfg, rule, files, opts)
}

// fakeRegistry serves the tags it is given and records what it was asked.
type fakeRegistry struct {
	tags []string
	err  error

	asked []oci.ChartRepo
	opts  []oci.FetchOptions
}

func (f *fakeRegistry) list(_ context.Context, repo oci.ChartRepo, opts oci.FetchOptions) ([]string, error) {
	f.asked = append(f.asked, repo)
	f.opts = append(f.opts, opts)
	return f.tags, f.err
}

func TestNextWorksOutTheVersion(t *testing.T) {
	cfg := loadRules(t)

	tests := []struct {
		name       string
		appVersion string
		tags       []string
		want       string
	}{
		{
			name:       "a new upstream version: the next W",
			appVersion: "v1.5.4",
			tags:       []string{"111.0.0_up1.5.2", "111.0.1_up1.5.3"},
			want:       "111.0.2+up1.5.4",
		},
		{
			name:       "the leading v of the appVersion is stripped",
			appVersion: "v1.5.4",
			tags:       nil,
			want:       "111.0.0+up1.5.4",
		},
		{
			name:       "an appVersion with no v",
			appVersion: "1.5.4",
			tags:       []string{"111.0.0_up1.5.3"},
			want:       "111.0.1+up1.5.4",
		},
		{
			name:       "the second candidate keeps Z.W",
			appVersion: "v1.5.4-rc.2",
			tags:       []string{"111.0.0_up1.5.3", "111.0.1_up1.5.4-rc.1"},
			want:       "111.0.1+up1.5.4-rc.2",
		},
		{
			name:       "the final keeps Z.W of its candidates",
			appVersion: "v1.5.4",
			tags:       []string{"111.0.0_up1.5.3", "111.0.1_up1.5.4-rc.1", "111.0.1_up1.5.4-rc.2"},
			want:       "111.0.1+up1.5.4",
		},
		{
			name:       "other majors are left out",
			appVersion: "v1.5.4",
			tags:       []string{"110.3.0_up1.5.3", "112.0.0_up2.0.0"},
			want:       "111.0.0+up1.5.4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
			edit(t, dir, "Chart.yaml", "appVersion: v1.5.4", "appVersion: "+tt.appVersion)
			registry := &fakeRegistry{tags: tt.tags}

			got, _, err := next(t, cfg, dir, Options{List: registry.list})

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNextReturnsTheVersionsThatAreReleased(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
	registry := &fakeRegistry{tags: []string{"111.0.0_up1.5.2", "111.0.1_up1.5.3"}}

	_, released, err := next(t, cfg, dir, Options{List: registry.list})

	require.NoError(t, err)
	require.Equal(t, []string{"111.0.0+up1.5.2", "111.0.1+up1.5.3"}, released, "the tags are turned back into versions")
}

func TestNextReadsTheRegistryOfTheEdition(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")

	t.Run("the staging registry of the edition, with the name of the chart", func(t *testing.T) {
		registry := &fakeRegistry{}

		_, _, err := next(t, cfg, dir, Options{List: registry.list})

		require.NoError(t, err)
		require.Equal(t, []oci.ChartRepo{{Host: "stgregistry.suse.com", Repository: "rancher/catalog/rancher-compliance"}}, registry.asked)
		require.Equal(t, []oci.FetchOptions{{PlainHTTP: false}}, registry.opts)
	})

	t.Run("a registry given replaces it, and the name is added", func(t *testing.T) {
		registry := &fakeRegistry{}

		_, _, err := next(t, cfg, dir, Options{Registry: "127.0.0.1:5052/e2e/", PlainHTTP: true, List: registry.list})

		require.NoError(t, err)
		require.Equal(t, []oci.ChartRepo{{Host: "127.0.0.1:5052", Repository: "e2e/rancher-compliance"}}, registry.asked)
		require.Equal(t, []oci.FetchOptions{{PlainHTTP: true}}, registry.opts)
	})

	t.Run("a repository that does not exist has no versions", func(t *testing.T) {
		registry := &fakeRegistry{err: fmt.Errorf("it is not there: %w", oci.ErrRepositoryNotFound)}

		got, released, err := next(t, cfg, dir, Options{List: registry.list})

		require.NoError(t, err)
		require.Equal(t, "111.0.0+up1.5.4", got)
		require.Empty(t, released)
	})

	t.Run("the credentials are asked for the host of the registry", func(t *testing.T) {
		registry := &fakeRegistry{}
		var asked string
		credentials := oci.CredentialFunc(func(host string) (string, string, error) {
			asked = host
			return "reader", "secret", nil
		})

		_, _, err := next(t, cfg, dir, Options{Credentials: credentials, List: registry.list})

		require.NoError(t, err)
		require.Equal(t, "stgregistry.suse.com", asked)
		require.Equal(t, []oci.FetchOptions{{Username: "reader", Password: "secret"}}, registry.opts)
	})

	t.Run("credentials that cannot be read stop it before the registry is asked", func(t *testing.T) {
		registry := &fakeRegistry{}
		credentials := oci.CredentialFunc(func(string) (string, string, error) {
			return "", "", errors.New("the saved login cannot be read")
		})

		_, _, err := next(t, cfg, dir, Options{Credentials: credentials, List: registry.list})

		require.ErrorContains(t, err, "the saved login cannot be read")
		require.Empty(t, registry.asked)
	})
}

func TestNextCannotWorkOutTheVersion(t *testing.T) {
	cfg := loadRules(t)

	tests := []struct {
		name    string
		chart   func(t *testing.T) string
		fake    *fakeRegistry
		wantErr string
	}{
		{
			name: "the registry fails",
			chart: func(t *testing.T) string {
				return copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
			},
			fake:    &fakeRegistry{err: errors.New("the registry asks for authentication")},
			wantErr: "the registry asks for authentication",
		},
		{
			name: "a published tag that is not a version",
			chart: func(t *testing.T) string {
				return copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
			},
			fake:    &fakeRegistry{tags: []string{"111.0.0_up1.5.3", "latest"}},
			wantErr: `version "latest" must look like`,
		},
		{
			name: "the same candidate is published",
			chart: func(t *testing.T) string {
				dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
				edit(t, dir, "Chart.yaml", "appVersion: v1.5.4", "appVersion: v1.5.4-rc.1")
				return dir
			},
			fake:    &fakeRegistry{tags: []string{"111.0.0_up1.5.4-rc.1"}},
			wantErr: "version 111.0.0+up1.5.4-rc.1 is already published",
		},
		{
			name: "an appVersion that is a placeholder",
			chart: func(t *testing.T) string {
				return copyChart(t, "rancher-webhook-upstream", "rancher-webhook")
			},
			fake:    &fakeRegistry{},
			wantErr: `the upstream version "999" does not make a valid version`,
		},
		{
			name: "no appVersion",
			chart: func(t *testing.T) string {
				dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
				edit(t, dir, "Chart.yaml", "appVersion: v1.5.4\n", "")
				return dir
			},
			fake:    &fakeRegistry{},
			wantErr: "the Chart.yaml has no appVersion",
		},
		{
			name: "a CRD chart has no rancher-version",
			chart: func(t *testing.T) string {
				return copyChart(t, "fleet", "fleet-crd")
			},
			fake:    &fakeRegistry{},
			wantErr: "a CRD chart has no rancher-version, so its version cannot be worked out on its own",
		},
		{
			name: "a chart for Rancher 2.15",
			chart: func(t *testing.T) string {
				dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
				edit(t, dir, "Chart.yaml", ">= 2.16.0-0 < 2.17.0-0", ">= 2.15.0-0 < 2.16.0-0")
				return dir
			},
			fake:    &fakeRegistry{},
			wantErr: "2.15 and before are not",
		},
		{
			name: "a Rancher version the rules have no major for",
			chart: func(t *testing.T) string {
				dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
				edit(t, dir, "Chart.yaml", ">= 2.16.0-0 < 2.17.0-0", ">= 2.99.0-0 < 2.100.0-0")
				return dir
			},
			fake:    &fakeRegistry{},
			wantErr: "the rules file has no chart major for Rancher 2.99",
		},
		{
			name: "no rancher-version",
			chart: func(t *testing.T) string {
				dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
				edit(t, dir, "Chart.yaml", "  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'\n", "")
				return dir
			},
			fake:    &fakeRegistry{},
			wantErr: "has no lower bound",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, released, err := next(t, cfg, tt.chart(t), Options{List: tt.fake.list})

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, got)
			require.Empty(t, released)
		})
	}
}

func TestNextWorksOutThePlainVersion(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "system-upgrade-controller", "system-upgrade-controller")
	edit(t, dir, "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.16.0-0 < 2.17.0-0")
	registry := &fakeRegistry{tags: []string{"111.0.3", "111.0.9", "110.9.9"}}

	got, _, err := next(t, cfg, dir, Options{List: registry.list})

	require.NoError(t, err)
	require.Equal(t, "111.0.10", got)
}

func TestSupportedRancher(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		want    string
		wantErr string
	}{
		{name: "a supported version", from: ">= 2.16.0-0 < 2.17.0-0", want: "2.16"},
		{name: "a later version", from: ">= 2.17.0-0 < 2.18.0-0", want: "2.17"},
		{name: "a version the new system does not support", from: ">= 2.15.0-0 < 2.16.0-0", wantErr: "2.15 and before are not"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
			edit(t, dir, "Chart.yaml", ">= 2.16.0-0 < 2.17.0-0", tt.from)
			files, err := chart.Load(dir)
			require.NoError(t, err)

			got, err := SupportedRancher(files)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// registries answers with the tags of each repository, such as
// rancher/catalog/fleet, and says that a repository it does not have is not
// found. It records the repositories it was asked about.
type registries struct {
	tags  map[string][]string
	errs  map[string]error
	asked []string
}

func (r *registries) list(_ context.Context, repo oci.ChartRepo, _ oci.FetchOptions) ([]string, error) {
	r.asked = append(r.asked, repo.Repository)
	if err := r.errs[repo.Repository]; err != nil {
		return nil, err
	}
	tags, ok := r.tags[repo.Repository]
	if !ok {
		return nil, fmt.Errorf("%s was not found in the registry: %w", repo, oci.ErrRepositoryNotFound)
	}
	return tags, nil
}

const (
	agentRepo = "rancher/catalog/fleet-agent"
	crdRepo   = "rancher/catalog/fleet-crd"
)

func TestCheckReleased(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)

	tests := []struct {
		name         string
		mainReleased []string
		registries   *registries
		wantErr      []string
	}{
		{
			name:         "every chart is at the same version",
			mainReleased: []string{"112.0.0+up0.16.2", "112.0.1+up0.16.3"},
			registries: &registries{tags: map[string][]string{
				agentRepo: {"112.0.0_up0.16.2", "112.0.1_up0.16.3"},
				crdRepo:   {"112.0.1_up0.16.3"},
			}},
		},
		{
			name:         "the order of the tags does not matter",
			mainReleased: []string{"112.0.1+up0.16.3", "112.0.0+up0.16.2"},
			registries: &registries{tags: map[string][]string{
				agentRepo: {"112.0.1_up0.16.3", "112.0.0_up0.16.2"},
				crdRepo:   {"112.0.0_up0.16.2", "112.0.1_up0.16.3"},
			}},
		},
		{
			name:         "a family that was never pushed",
			mainReleased: nil,
			registries:   &registries{},
		},
		{
			name:         "one chart is behind",
			mainReleased: []string{"112.0.1+up0.16.3"},
			registries: &registries{tags: map[string][]string{
				agentRepo: {"112.0.1_up0.16.3"},
				crdRepo:   {"112.0.0_up0.16.2"},
			}},
			wantErr: []string{
				`the charts of the family "fleet" are not at the same released version`,
				"fleet 112.0.1+up0.16.3, fleet-agent 112.0.1+up0.16.3, fleet-crd 112.0.0+up0.16.2",
			},
		},
		{
			name:         "one chart has nothing released",
			mainReleased: []string{"112.0.1+up0.16.3"},
			registries: &registries{tags: map[string][]string{
				agentRepo: {"112.0.1_up0.16.3"},
			}},
			wantErr: []string{"fleet-crd (nothing released)"},
		},
		{
			name:         "the main chart has nothing released and the others do",
			mainReleased: nil,
			registries: &registries{tags: map[string][]string{
				agentRepo: {"112.0.1_up0.16.3"},
				crdRepo:   {"112.0.1_up0.16.3"},
			}},
			wantErr: []string{"fleet (nothing released)"},
		},
		{
			name:         "a registry fails and the others are still read",
			mainReleased: []string{"112.0.1+up0.16.3"},
			registries: &registries{
				tags: map[string][]string{crdRepo: {"112.0.1_up0.16.3"}},
				errs: map[string]error{agentRepo: errors.New("the registry asks for authentication")},
			},
			wantErr: []string{"fleet-agent: the registry asks for authentication"},
		},
		{
			name:         "a tag that is not a version",
			mainReleased: []string{"112.0.1+up0.16.3"},
			registries: &registries{tags: map[string][]string{
				agentRepo: {"112.0.1_up0.16.3", "latest"},
				crdRepo:   {"112.0.1_up0.16.3"},
			}},
			wantErr: []string{"fleet-agent", `"latest"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckReleased(context.Background(), cfg, members, tt.mainReleased, Options{List: tt.registries.list})

			if len(tt.wantErr) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range tt.wantErr {
				require.ErrorContains(t, err, want)
			}
		})
	}
}

func TestCheckReleasedReadsTheOtherCharts(t *testing.T) {
	cfg := loadRules(t)
	members, err := cfg.Family("fleet")
	require.NoError(t, err)
	registry := &registries{tags: map[string][]string{
		agentRepo: {"112.0.1_up0.16.3"},
		crdRepo:   {"112.0.1_up0.16.3"},
	}}

	err = CheckReleased(context.Background(), cfg, members, []string{"112.0.1+up0.16.3"}, Options{List: registry.list})

	require.NoError(t, err)
	require.Equal(t, []string{agentRepo, crdRepo}, registry.asked, "the main chart was read by the caller, so only the others are asked, in order")
}
