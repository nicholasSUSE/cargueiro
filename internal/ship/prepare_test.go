package ship

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/annotations"
	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
)

const manifestDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func loadRules(t *testing.T) *rules.Config {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "rules.yaml"))
	require.NoError(t, err)
	cfg, err := rules.Parse(data)
	require.NoError(t, err)
	return cfg
}

// tgz builds a chart archive from the files of a chart called name.
func tgz(t *testing.T, name string, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for file, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name + "/" + file, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func readFixture(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "data", "charts", path))
	require.NoError(t, err)
	return string(data)
}

const (
	complianceVersion = "111.0.0+up1.5.4"
	complianceTag     = "111.0.0_up1.5.4"
	complianceIcon    = "https://charts.rancher.io/assets/logos/rancher-compliance.svg"
)

// complianceArchive is the compliance chart as a chart owner would push it once
// it is stamped: with its final version and the images it needs listed, and its
// icon on the web. edit changes the text of Chart.yaml before the archive is made.
func complianceArchive(t *testing.T, edit func(string) string) []byte {
	t.Helper()

	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Chart.yaml":  readFixture(t, "rancher-compliance-upstream/src/rancher-compliance/Chart.yaml"),
		"values.yaml": readFixture(t, "rancher-compliance-upstream/src/rancher-compliance/values.yaml"),
	})
	files, err := chart.Load(dir)
	require.NoError(t, err)
	images, _, problems := annotations.GenerateHelmImages(files.Annotations, files.Values)
	require.Empty(t, problems)

	chartYAML, err := chart.SetVersion([]byte(readFixture(t, "rancher-compliance-upstream/src/rancher-compliance/Chart.yaml")), complianceVersion)
	require.NoError(t, err)
	chartYAML, err = chart.SetAnnotation(chartYAML, "helm.sh/images", images)
	require.NoError(t, err)

	text := string(chartYAML)
	if edit != nil {
		text = edit(text)
	}
	return tgz(t, "rancher-compliance", map[string]string{
		"Chart.yaml":  text,
		"values.yaml": readFixture(t, "rancher-compliance-upstream/src/rancher-compliance/values.yaml"),
	})
}

// fakes stands in for the registry and the web.
type fakes struct {
	archive    []byte
	resolveErr error
	pullErr    error
	logo       Logo
	logoErr    error

	resolved []oci.ChartRef
	pulled   []string
	logos    []string
}

func (f *fakes) options() Options {
	return Options{
		Resolve: func(_ context.Context, ref oci.ChartRef, _ oci.FetchOptions) (string, error) {
			f.resolved = append(f.resolved, ref)
			return manifestDigest, f.resolveErr
		},
		Pull: func(_ context.Context, repo oci.ChartRepo, digest string, _ oci.FetchOptions) ([]byte, error) {
			f.pulled = append(f.pulled, repo.String()+"@"+digest)
			return f.archive, f.pullErr
		},
		Logo: func(_ context.Context, address string) (Logo, error) {
			f.logos = append(f.logos, address)
			return f.logo, f.logoErr
		},
	}
}

func complianceRef() oci.ChartRef {
	return oci.ChartRef{Host: "127.0.0.1:5052", Repository: "e2e/rancher-compliance", Tag: complianceTag}
}

func TestPrepareAMainChart(t *testing.T) {
	f := &fakes{archive: complianceArchive(t, nil), logo: Logo{Data: []byte("<svg/>"), Extension: ".svg"}}
	dir := t.TempDir()

	got, err := Prepare(context.Background(), loadRules(t), complianceRef(), dir, f.options())

	require.NoError(t, err)
	require.Equal(t, "rancher-compliance", got.Chart)
	require.Equal(t, complianceVersion, got.Version)
	require.Equal(t, &Logo{Data: []byte("<svg/>"), Extension: ".svg"}, got.Logo)
	require.Equal(t, []string{complianceIcon}, f.logos)
	require.Equal(t, []string{"127.0.0.1:5052/e2e/rancher-compliance@" + manifestDigest}, f.pulled, "the archive is read by the digest the tag resolved to")
	entry := string(got.Entry)
	require.Contains(t, entry, "\nicon: file://assets/logos/rancher-compliance.svg\n")
	require.Contains(t, entry, "\nurls:\n- oci://127.0.0.1:5052/e2e/rancher-compliance:"+complianceVersion+"@"+manifestDigest+"\n")
	require.Contains(t, entry, "\nversion: "+complianceVersion+"\n")
}

func TestPrepareDoesNotWriteAnything(t *testing.T) {
	f := &fakes{archive: complianceArchive(t, nil), logo: Logo{Data: []byte("<svg/>"), Extension: ".svg"}}
	dir := t.TempDir()

	_, err := Prepare(context.Background(), loadRules(t), complianceRef(), dir, f.options())

	require.NoError(t, err)
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	require.Empty(t, entries)
}

func TestPrepareUsesTheLogoTheRepositoryHas(t *testing.T) {
	f := &fakes{archive: complianceArchive(t, nil)}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"assets/logos/rancher-compliance.png": "the logo"})

	got, err := Prepare(context.Background(), loadRules(t), complianceRef(), dir, f.options())

	require.NoError(t, err)
	require.Nil(t, got.Logo)
	require.Empty(t, f.logos, "nothing is downloaded")
	require.Contains(t, string(got.Entry), "\nicon: file://assets/logos/rancher-compliance.png\n")
}

func TestPrepareAnExemptChart(t *testing.T) {
	published, err := os.ReadFile(filepath.Join("..", "..", "tests", "data", "charts", "fleet", "published", "fleet-110.0.3+up0.16.3.tgz"))
	require.NoError(t, err)
	f := &fakes{archive: published}
	ref := oci.ChartRef{Host: "127.0.0.1:5052", Repository: "e2e/fleet", Tag: "110.0.3_up0.16.3"}

	got, err := Prepare(context.Background(), loadRules(t), ref, t.TempDir(), f.options())

	require.NoError(t, err)
	require.Nil(t, got.Logo)
	require.Empty(t, f.logos, "an exempt chart needs no logo")
	require.Contains(t, string(got.Entry), "\nicon: https://charts.rancher.io/assets/logos/fleet.svg\n", "and keeps its icon")
	require.Contains(t, string(got.Entry), "\nurls:\n- oci://127.0.0.1:5052/e2e/fleet:110.0.3+up0.16.3@"+manifestDigest+"\n")
}

func TestPrepareFails(t *testing.T) {
	archive := complianceArchive(t, nil)
	logo := Logo{Data: []byte("x"), Extension: ".svg"}
	iconLine := "icon: " + complianceIcon + "\n"

	tests := []struct {
		name    string
		fakes   *fakes
		ref     oci.ChartRef
		wantErr string
	}{
		{
			name:    "the tag cannot be resolved",
			fakes:   &fakes{resolveErr: errors.New("the registry is down")},
			ref:     complianceRef(),
			wantErr: "the registry is down",
		},
		{
			name:    "the archive cannot be pulled",
			fakes:   &fakes{pullErr: errors.New("not found")},
			ref:     complianceRef(),
			wantErr: "pull the archive",
		},
		{
			name:    "what is pulled is not an archive",
			fakes:   &fakes{archive: []byte("not an archive")},
			ref:     complianceRef(),
			wantErr: "the archive is not gzip",
		},
		{
			name:    "the tag is not the version of the chart",
			fakes:   &fakes{archive: archive, logo: logo},
			ref:     oci.ChartRef{Host: "127.0.0.1:5052", Repository: "e2e/rancher-compliance", Tag: "111.0.1_up1.5.5"},
			wantErr: "the reference says version 111.0.1+up1.5.5 but the chart is " + complianceVersion,
		},
		{
			name:    "the repository is not named after the chart",
			fakes:   &fakes{archive: archive, logo: logo},
			ref:     oci.ChartRef{Host: "127.0.0.1:5052", Repository: "e2e/other", Tag: complianceTag},
			wantErr: `the repository "e2e/other" is not named after the chart "rancher-compliance"`,
		},
		{
			name:    "a chart that is not in the rules",
			fakes:   &fakes{archive: tgz(t, "hello", map[string]string{"Chart.yaml": "apiVersion: v2\nname: hello\nversion: 1.0.0\n"})},
			ref:     oci.ChartRef{Host: "127.0.0.1:5052", Repository: "e2e/hello", Tag: "1.0.0"},
			wantErr: `chart "hello" is not listed in the rules file`,
		},
		{
			name: "a chart with no icon that needs one",
			fakes: &fakes{archive: complianceArchive(t, func(s string) string {
				return strings.Replace(s, iconLine, "", 1)
			})},
			ref:     complianceRef(),
			wantErr: "rancher-compliance has no icon, and a chart that is not exempt needs one",
		},
		{
			name: "an icon that is a local file already",
			fakes: &fakes{archive: complianceArchive(t, func(s string) string {
				return strings.Replace(s, iconLine, "icon: file://assets/logos/rancher-compliance.svg\n", 1)
			})},
			ref:     complianceRef(),
			wantErr: `the icon "file://assets/logos/rancher-compliance.svg" of rancher-compliance is a local file already`,
		},
		{
			name:    "a logo that cannot be downloaded",
			fakes:   &fakes{archive: archive, logoErr: errors.New("the server answered 404")},
			ref:     complianceRef(),
			wantErr: "the server answered 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Prepare(context.Background(), loadRules(t), tt.ref, t.TempDir(), tt.fakes.options())

			require.ErrorContains(t, err, tt.wantErr)
			require.Equal(t, Shipment{}, got)
		})
	}
}

func TestPrepareAChartThatBreaksTheRules(t *testing.T) {
	broken := complianceArchive(t, func(s string) string {
		return strings.Replace(s, "catalog.cattle.io/certified: rancher", "catalog.cattle.io/certified: someone", 1)
	})
	f := &fakes{archive: broken, logo: Logo{Data: []byte("x"), Extension: ".svg"}}

	got, err := Prepare(context.Background(), loadRules(t), complianceRef(), t.TempDir(), f.options())

	var invalid *InvalidChartError
	require.ErrorAs(t, err, &invalid)
	require.False(t, invalid.Result.Valid)
	require.Equal(t, "rancher-compliance", invalid.Result.Chart)
	require.ErrorContains(t, err, "rancher-compliance "+complianceVersion+" does not follow the chart rules")
	require.Equal(t, Shipment{}, got)
	require.Empty(t, f.logos, "nothing is downloaded for a chart that is refused")
}
