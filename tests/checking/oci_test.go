//go:build e2e

package checking

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/rancher/cargueiro/internal/check"
	"github.com/rancher/cargueiro/tests/internal/harness"
)

const fleetVersion = "110.0.3+up0.16.3"

// openRegistry logs a session in to the registry that asks for no password
// (the registry accepts any login, and says so) and returns the registry
// address and the session, which pushes there.
func openRegistry(t *testing.T) (string, harness.Session) {
	t.Helper()

	address := harness.OpenRegistry(t)
	session := harness.NewSession(t)
	res := session.Login(t, address, []string{"--plain-http"})
	require.Zero(t, res.ExitCode, res.Stderr)
	return address, session
}

// pushArchive pushes a chart archive under <address>/rancher.
func pushArchive(t *testing.T, session harness.Session, address, archive string) {
	t.Helper()

	res := session.Push(t, address+"/rancher", archive)
	require.Zero(t, res.ExitCode, res.Stderr)
}

// fixtureArchive packages the two files kept in tests/data for a chart as a
// chart archive, and returns its path.
func fixtureArchive(t *testing.T, dataDir, name string) string {
	t.Helper()

	src := harness.ChartSource(t, dataDir, name)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, file := range []string{"Chart.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join(src, file))
		require.NoError(t, err)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name + "/" + file,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(data)),
		}))
		_, err = tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	path := filepath.Join(t.TempDir(), name+".tgz")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

// checkOCI runs check oci against the open registry.
func checkOCI(t *testing.T, args ...string) harness.Result {
	t.Helper()

	return harness.Cargueiro(t, t.TempDir(), append([]string{"check", "oci"}, args...)...)
}

// checkChartText runs check chart on a chart directory.
func checkChartText(t *testing.T, dir string, args ...string) harness.Result {
	t.Helper()

	return harness.Cargueiro(t, t.TempDir(), append([]string{"check", "chart", dir}, args...)...)
}

func TestCheckOCIFleetCharts(t *testing.T) {
	address, session := openRegistry(t)

	for _, name := range []string{"fleet-crd", "fleet", "fleet-agent"} {
		t.Run(name, func(t *testing.T) {
			pushArchive(t, session, address, harness.FleetPublished(t, name, fleetVersion))
			ref := address + "/rancher/" + name + ":" + fleetVersion

			res := checkOCI(t, ref, "--plain-http", "--output", "json")
			require.Zero(t, res.ExitCode, res.Stderr)
			require.Empty(t, res.Stderr)
			var got check.Result
			require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got), res.Stdout)
			require.True(t, got.Valid)
			require.Equal(t, name, got.Chart)

			// The same chart gives the same report from a registry and from a directory.
			fromRegistry := checkOCI(t, ref, "--plain-http")
			fromDirectory := checkChartText(t, harness.FleetSource(t, name))
			require.Zero(t, fromRegistry.ExitCode, fromRegistry.Stderr)
			require.Equal(t, fromDirectory.Stdout, fromRegistry.Stdout)
		})
	}
}

func TestCheckOCIBreaksTheRules(t *testing.T) {
	address, session := openRegistry(t)

	tests := []struct {
		name    string
		version string
		tag     string
	}{
		{
			name:    "system-upgrade-controller",
			version: "110.0.0",
			tag:     "110.0.0",
		},
		{
			name:    "longhorn",
			version: "110.1.0+up1.12.1",
			tag:     "110.1.0+up1.12.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pushArchive(t, session, address, fixtureArchive(t, tt.name, tt.name))
			ref := address + "/rancher/" + tt.name + ":" + tt.tag

			fromRegistry := checkOCI(t, ref, "--plain-http")
			fromDirectory := checkChartText(t, harness.ChartSource(t, tt.name, tt.name))

			require.Equal(t, 1, fromRegistry.ExitCode)
			require.Contains(t, fromRegistry.Stderr, "chart is not valid")
			require.Contains(t, fromRegistry.Stdout, "valid: false\n")
			require.Equal(t, fromDirectory.Stdout, fromRegistry.Stdout)
		})
	}

	t.Run("--no-error-hip-15 for a chart that only breaks the HIP-15 rules", func(t *testing.T) {
		ref := address + "/rancher/system-upgrade-controller:110.0.0"

		res := checkOCI(t, ref, "--plain-http", "--no-error-hip-15")

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Contains(t, res.Stdout, "hip15-annotations: invalid\n")
		require.Equal(t, "chart is not valid (HIP-15 errors ignored by --no-error-hip-15)\n", res.Stderr)
	})
}

// pushNotAChart pushes an artifact that is not a Helm chart.
func pushNotAChart(t *testing.T, address, repository, tag string) {
	t.Helper()
	ctx := context.Background()

	store := memory.New()
	data := []byte("not a chart")
	layer := ocispec.Descriptor{
		MediaType: "text/plain",
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}
	require.NoError(t, store.Push(ctx, layer, bytes.NewReader(data)))
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "application/vnd.example.thing", oras.PackManifestOptions{
		Layers: []ocispec.Descriptor{layer},
	})
	require.NoError(t, err)
	require.NoError(t, store.Tag(ctx, manifest, tag))

	repo, err := remote.NewRepository(address + "/" + repository)
	require.NoError(t, err)
	repo.PlainHTTP = true
	_, err = oras.Copy(ctx, store, tag, repo, tag, oras.DefaultCopyOptions)
	require.NoError(t, err)
}

func TestCheckOCIFailsWithoutPrinting(t *testing.T) {
	address, session := openRegistry(t)
	pushArchive(t, session, address, harness.FleetPublished(t, "fleet", fleetVersion))
	pushNotAChart(t, address, "rancher/notachart", "1.0.0")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "the tag does not exist",
			args:    []string{address + "/rancher/fleet:9.9.9", "--plain-http"},
			wantErr: "was not found in the registry",
		},
		{
			name:    "the repository does not exist",
			args:    []string{address + "/rancher/nothing-here:1.0.0", "--plain-http"},
			wantErr: "was not found in the registry",
		},
		{
			name:    "it is not a Helm chart",
			args:    []string{address + "/rancher/notachart:1.0.0", "--plain-http"},
			wantErr: "is not a Helm chart",
		},
		{
			name:    "nothing listens there",
			args:    []string{"127.0.0.1:1/rancher/fleet:" + fleetVersion, "--plain-http"},
			wantErr: "fetch 127.0.0.1:1/rancher/fleet",
		},
		{
			name:    "the registry speaks HTTP and HTTPS is the default",
			args:    []string{address + "/rancher/fleet:" + fleetVersion},
			wantErr: "fetch " + address + "/rancher/fleet",
		},
		{
			name:    "no tag",
			args:    []string{address + "/rancher/fleet", "--plain-http"},
			wantErr: "has no tag",
		},
		{
			name:    "a digest",
			args:    []string{address + "/rancher/fleet@sha256:de5ef718a024560f857aad82cb95624067c13d0248455d498a25bea5cb9bc742", "--plain-http"},
			wantErr: "references by digest",
		},
		{
			name:    "an unknown output format",
			args:    []string{address + "/rancher/fleet:" + fleetVersion, "--plain-http", "--output", "yaml"},
			wantErr: `unknown output format "yaml"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := checkOCI(t, tt.args...)

			require.Equal(t, 1, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
		})
	}
}
