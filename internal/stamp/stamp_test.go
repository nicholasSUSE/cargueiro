package stamp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/check"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
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

// diffLines returns the lines of after that before does not have, and the lines
// of before that after does not have, as long as the common lines keep their
// order.
func diffLines(t *testing.T, before, after string) (removed, added []string) {
	t.Helper()

	b, a := strings.Split(before, "\n"), strings.Split(after, "\n")
	i, j := 0, 0
	for i < len(b) && j < len(a) {
		switch {
		case b[i] == a[j]:
			i++
			j++
		case !contains(a[j:], b[i]):
			removed = append(removed, b[i])
			i++
		default:
			added = append(added, a[j])
			j++
		}
	}
	removed = append(removed, b[i:]...)
	added = append(added, a[j:]...)
	return removed, added
}

func contains(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

func TestBuildStampsTheComplianceChart(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")

	plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up1.5.4"})

	require.NoError(t, err)
	require.True(t, plan.Changed())
	require.Equal(t, "1.5.4", plan.OldVersion)
	require.Equal(t, "111.0.0+up1.5.4", plan.NewVersion)
	require.Equal(t, []string{"kuberlr-kubectl", "compliance-operator", "security-scan", "mirrored-sonobuoy-sonobuoy"}, plan.AddedImages)

	// Only the version line and the new annotation differ.
	removed, added := diffLines(t, string(plan.Before), string(plan.After))
	require.Equal(t, []string{"version: 1.5.4"}, removed)
	require.Equal(t, []string{
		"  helm.sh/images: |",
		"    - name: kuberlr-kubectl",
		"      image: rancher/kuberlr-kubectl:v8.1.2",
		"    - name: compliance-operator",
		"      image: rancher/compliance-operator:v1.5.4",
		"    - name: security-scan",
		"      image: rancher/security-scan:v0.10.4",
		"    - name: mirrored-sonobuoy-sonobuoy",
		"      image: rancher/mirrored-sonobuoy-sonobuoy:v0.57.5",
		"version: 111.0.0+up1.5.4",
	}, trimBlank(added))

	// Building writes nothing.
	onDisk, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.Equal(t, string(plan.Before), string(onDisk))
}

func trimBlank(lines []string) []string {
	var out []string
	for _, l := range lines {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestWriteAndTheResultPassesCheck(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
	require.NoError(t, os.Chmod(filepath.Join(dir, "Chart.yaml"), 0o640))

	plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up1.5.4"})
	require.NoError(t, err)
	require.NoError(t, plan.Write())

	written, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.Equal(t, string(plan.After), string(written))
	info, err := os.Stat(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o640), info.Mode().Perm(), "the mode of the file is kept")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2, "no temporary file is left behind")

	// The chart that stamp made is a valid one for check.
	files, err := chart.Load(dir)
	require.NoError(t, err)
	result, err := check.Chart(cfg, files, files.Metadata.Version)
	require.NoError(t, err)
	require.True(t, result.Valid, "%+v", result)

	// Stamping it again with the same version changes nothing.
	again, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up1.5.4"})
	require.NoError(t, err)
	require.False(t, again.Changed())
	require.Empty(t, again.AddedImages)
}

func TestBuildTheWebhookChart(t *testing.T) {
	cfg := loadRules(t)

	t.Run("as upstream keeps it, the tag latest is refused", func(t *testing.T) {
		dir := copyChart(t, "rancher-webhook-upstream", "rancher-webhook")

		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.4+up0.11.4"})

		require.EqualError(t, err, `values.yaml image: "tag" must not be "latest"`)
		require.Nil(t, plan)
	})

	t.Run("with the real tag the owner's automation sets", func(t *testing.T) {
		dir := copyChart(t, "rancher-webhook-upstream", "rancher-webhook")
		edit(t, dir, "values.yaml", "tag: latest", "tag: v0.11.4")

		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.4+up0.11.4"})

		require.NoError(t, err)
		require.Equal(t, "999", plan.OldVersion)
		require.Equal(t, []string{"rancher-webhook"}, plan.AddedImages)
		removed, added := diffLines(t, string(plan.Before), string(plan.After))
		require.Equal(t, []string{`version: "999"`}, removed)
		require.Contains(t, added, "version: 111.0.4+up0.11.4")
		require.Contains(t, added, "      image: rancher/rancher-webhook:v0.11.4")
	})
}

func TestBuildRefusesTheVersion(t *testing.T) {
	cfg := loadRules(t)

	tests := []struct {
		name    string
		version string
		wantErr string
	}{
		{
			name:    "no +up part",
			version: "111.0.0",
			wantErr: `version "111.0.0" must look like 1XY.Z.W+upA.B.C`,
		},
		{
			name:    "a prerelease that is not allowed",
			version: "111.0.0+up1.5.4-dev.1",
			wantErr: `version "111.0.0+up1.5.4-dev.1" must look like`,
		},
		{
			name:    "the major of another Rancher version",
			version: "112.0.0+up1.5.4",
			wantErr: `version "112.0.0+up1.5.4" has major 112, but charts for Rancher 2.16 have major 111`,
		},
		{
			name:    "the major of the old system",
			version: "110.3.0+up1.5.4",
			wantErr: `has major 110, but charts for Rancher 2.16 have major 111`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")

			plan, err := Build(context.Background(), cfg, dir, Options{Version: tt.version})

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, plan)
		})
	}
}

func TestBuildRefusesAChartForTheOldSystem(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
	edit(t, dir, "Chart.yaml", ">= 2.16.0-0 < 2.17.0-0", ">= 2.15.0-0 < 2.16.0-0")

	plan, err := Build(context.Background(), cfg, dir, Options{Version: "110.3.0+up1.5.4"})

	require.ErrorContains(t, err, "is for Rancher 2.15: Rancher 2.16 and later are supported, 2.15 and before are not")
	require.Nil(t, plan)
}

func TestBuildNeedsARancherVersion(t *testing.T) {
	cfg := loadRules(t)

	tests := []struct {
		name  string
		write func(t *testing.T, dir string)
	}{
		{
			name: "no annotation",
			write: func(t *testing.T, dir string) {
				edit(t, dir, "Chart.yaml", "  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'\n", "")
			},
		},
		{
			name: "no lower bound",
			write: func(t *testing.T, dir string) {
				edit(t, dir, "Chart.yaml", ">= 2.16.0-0 < 2.17.0-0", "< 2.17.0-0")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
			tt.write(t, dir)

			plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up1.5.4"})

			require.ErrorContains(t, err, "has no lower bound like \">= 2.16.0-0\"")
			require.Nil(t, plan)
		})
	}
}

func TestBuildAChartWithoutValues(t *testing.T) {
	cfg := loadRules(t)

	t.Run("a CRD chart: only the version, and its form is all that is checked", func(t *testing.T) {
		dir := copyChart(t, "fleet", "fleet-crd")
		require.NoError(t, os.Remove(filepath.Join(dir, "values.yaml")))

		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up0.16.3"})

		require.NoError(t, err)
		require.Empty(t, plan.AddedImages)
		removed, added := diffLines(t, string(plan.Before), string(plan.After))
		require.Equal(t, []string{"version: 110.0.3+up0.16.3"}, removed)
		require.Equal(t, []string{"version: 111.0.0+up0.16.3"}, added)
	})

	t.Run("a chart that is not a CRD gets no annotation", func(t *testing.T) {
		dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
		require.NoError(t, os.Remove(filepath.Join(dir, "values.yaml")))

		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up1.5.4"})

		require.NoError(t, err)
		require.Empty(t, plan.AddedImages)
		require.NotContains(t, string(plan.After), "helm.sh/images")
	})
}

func TestBuildKeepsAnAnnotationThatExists(t *testing.T) {
	cfg := loadRules(t)

	t.Run("and extends it with the images it does not list", func(t *testing.T) {
		dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
		edit(t, dir, "Chart.yaml", "annotations:\n", "annotations:\n  helm.sh/images: |\n    - name: operator\n      image: rancher/compliance-operator:v1.5.4\n    - name: hook\n      image: rancher/hook:v1\n")

		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up1.5.4"})

		require.NoError(t, err)
		require.Equal(t, []string{"kuberlr-kubectl", "security-scan", "mirrored-sonobuoy-sonobuoy"}, plan.AddedImages)
		require.Contains(t, string(plan.After), "    - name: operator\n      image: rancher/compliance-operator:v1.5.4\n    - name: hook\n      image: rancher/hook:v1\n    - name: kuberlr-kubectl\n      image: rancher/kuberlr-kubectl:v8.1.2\n    - name: security-scan\n")
	})

	t.Run("and refuses one that is not valid", func(t *testing.T) {
		dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
		edit(t, dir, "Chart.yaml", "annotations:\n", "annotations:\n  helm.sh/images: fleet\n")

		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up1.5.4"})

		require.ErrorContains(t, err, "helm.sh/images: must be a YAML list of entries with a name and an image")
		require.Nil(t, plan)
	})
}

func TestBuildPlainChart(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "system-upgrade-controller", "system-upgrade-controller")
	edit(t, dir, "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.16.0-0 < 2.17.0-0")

	t.Run("a version without a +up part", func(t *testing.T) {
		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0"})

		require.NoError(t, err)
		require.Equal(t, "111.0.0", plan.NewVersion)
	})

	t.Run("a version with one is refused", func(t *testing.T) {
		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up0.20.1"})

		require.ErrorContains(t, err, `version "111.0.0+up0.20.1" must look like 1XY.Z.W`)
		require.Nil(t, plan)
	})
}

func TestBuildFails(t *testing.T) {
	cfg := loadRules(t)

	t.Run("a chart that is not in the rules file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("name: epinio-extra\nversion: 1.0.0\n"), 0o600))

		plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.0+up1.0.0"})

		require.ErrorContains(t, err, `chart "epinio-extra" is not listed in the rules file`)
		require.Nil(t, plan)
	})

	t.Run("no Chart.yaml", func(t *testing.T) {
		plan, err := Build(context.Background(), cfg, t.TempDir(), Options{Version: "111.0.0+up1.0.0"})

		require.ErrorContains(t, err, "Chart.yaml not found")
		require.Nil(t, plan)
	})
}

func TestWriteDoesNothingWhenNothingChanges(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "fleet", "fleet-crd")
	plan, err := Build(context.Background(), cfg, dir, Options{Version: "110.0.3+up0.16.3"})
	require.NoError(t, err)
	require.False(t, plan.Changed())
	before, err := os.Stat(plan.Path)
	require.NoError(t, err)

	require.NoError(t, plan.Write())

	after, err := os.Stat(plan.Path)
	require.NoError(t, err)
	require.Equal(t, before.ModTime(), after.ModTime())
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

func TestBuildWorksOutTheVersion(t *testing.T) {
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

			plan, err := Build(context.Background(), cfg, dir, Options{List: registry.list})

			require.NoError(t, err)
			require.Equal(t, tt.want, plan.NewVersion)
			require.Contains(t, string(plan.After), "\nversion: "+tt.want+"\n")
		})
	}
}

func TestBuildReadsTheRegistryOfTheEdition(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")

	t.Run("the staging registry of the edition, with the name of the chart", func(t *testing.T) {
		registry := &fakeRegistry{}

		_, err := Build(context.Background(), cfg, dir, Options{List: registry.list})

		require.NoError(t, err)
		require.Equal(t, []oci.ChartRepo{{Host: "stgregistry.suse.com", Repository: "rancher/catalog/rancher-compliance"}}, registry.asked)
		require.Equal(t, []oci.FetchOptions{{PlainHTTP: false}}, registry.opts)
	})

	t.Run("a registry given replaces it, and the name is added", func(t *testing.T) {
		registry := &fakeRegistry{}

		_, err := Build(context.Background(), cfg, dir, Options{Registry: "127.0.0.1:5052/e2e/", PlainHTTP: true, List: registry.list})

		require.NoError(t, err)
		require.Equal(t, []oci.ChartRepo{{Host: "127.0.0.1:5052", Repository: "e2e/rancher-compliance"}}, registry.asked)
		require.Equal(t, []oci.FetchOptions{{PlainHTTP: true}}, registry.opts)
	})

	t.Run("a repository that does not exist has no versions", func(t *testing.T) {
		registry := &fakeRegistry{err: fmt.Errorf("it is not there: %w", oci.ErrRepositoryNotFound)}

		plan, err := Build(context.Background(), cfg, dir, Options{List: registry.list})

		require.NoError(t, err)
		require.Equal(t, "111.0.0+up1.5.4", plan.NewVersion)
	})
}

func TestBuildDoesNotReadTheRegistryForAGivenVersion(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "rancher-compliance-upstream", "rancher-compliance")
	registry := &fakeRegistry{err: errors.New("the registry must not be asked")}

	plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.9+up1.5.4", List: registry.list})

	require.NoError(t, err)
	require.Equal(t, "111.0.9+up1.5.4", plan.NewVersion)
	require.Empty(t, registry.asked)
}

func TestBuildCannotWorkOutTheVersion(t *testing.T) {
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
				dir := copyChart(t, "rancher-webhook-upstream", "rancher-webhook")
				edit(t, dir, "values.yaml", "tag: latest", "tag: v0.11.4")
				return dir
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
			wantErr: "a CRD chart has no rancher-version, so stamp cannot work out its version: give --version",
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
			dir := tt.chart(t)
			before, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
			require.NoError(t, err)

			plan, err := Build(context.Background(), cfg, dir, Options{List: tt.fake.list})

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, plan)
			after, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
			require.NoError(t, err)
			require.Equal(t, string(before), string(after))
		})
	}
}

func TestBuildWorksOutThePlainVersion(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "system-upgrade-controller", "system-upgrade-controller")
	edit(t, dir, "Chart.yaml", ">= 2.15.0-0 < 2.16.0-0", ">= 2.16.0-0 < 2.17.0-0")
	registry := &fakeRegistry{tags: []string{"111.0.3", "111.0.9", "110.9.9"}}

	plan, err := Build(context.Background(), cfg, dir, Options{List: registry.list})

	require.NoError(t, err)
	require.Equal(t, "111.0.10", plan.NewVersion)
}
