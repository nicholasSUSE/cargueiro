//go:build e2e

// Package checking holds the e2e tests of `cargueiro check chart`, run against
// the real binary and real SUSE charts.
package checking

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/check"
	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
)

func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// unlisted is the error for an image in values.yaml that helm.sh/images does
// not list.
func unlisted(path, image string) string {
	return fmt.Sprintf("values.yaml %s: image %q is not listed in helm.sh/images", path, image)
}

// checkJSON runs check chart on a chart directory and decodes the JSON result.
func checkJSON(t *testing.T, dir string) (check.Result, harness.Result) {
	t.Helper()

	res := harness.Cargueiro(t, t.TempDir(), "check", "chart", dir, "--output", "json")

	var got check.Result
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got), "stdout: %s\nstderr: %s", res.Stdout, res.Stderr)
	return got, res
}

// TestFleetIsValid checks the charts that carry a valid helm.sh/images
// annotation: nothing is reported and the exit code is 0.
func TestFleetIsValid(t *testing.T) {
	tests := []struct {
		chart       string
		kind        string
		kubeVersion string
	}{
		{
			chart:       "fleet",
			kind:        "main",
			kubeVersion: ">= 1.34.0-0 < 1.37.0-0",
		},
		{
			// A crd chart does not carry the annotation.
			chart:       "fleet-crd",
			kind:        "crd",
			kubeVersion: "",
		},
		{
			chart:       "fleet-agent",
			kind:        "extra",
			kubeVersion: ">= 1.34.0-0 < 1.37.0-0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.chart, func(t *testing.T) {
			got, res := checkJSON(t, harness.FleetSource(t, tt.chart))

			require.Zero(t, res.ExitCode, res.Stderr)
			require.Empty(t, res.Stderr)
			require.Equal(t, tt.chart, got.Chart)
			require.Equal(t, tt.kind, got.Kind)
			require.Equal(t, "fleet", got.Family)
			require.Equal(t, "110.0.3+up0.16.3", got.Metadata.Version)
			require.Equal(t, tt.kubeVersion, got.Metadata.KubeVersion)
			require.True(t, got.Valid)
			require.Equal(t, check.Section{Valid: true, Errors: []string{}}, got.Annotations)
			require.Equal(t, check.Section{Valid: true, Errors: []string{}}, got.HIP15Annotations)
		})
	}
}

// TestRancherChartsBreakTheRules checks charts that are released today without
// a helm.sh/images annotation, and two of them with a kube-version that is not
// a range. Each is printed, with every error, and the exit code is 1.
func TestRancherChartsBreakTheRules(t *testing.T) {
	tests := []struct {
		name            string
		dataDir         string
		chart           string
		kind            string
		family          string
		wantAnnotations []string
		wantHIP15       []string
	}{
		{
			name:            "rancher-monitoring has 10 images that are not listed",
			dataDir:         "rancher-monitoring",
			chart:           "rancher-monitoring",
			kind:            "main",
			family:          "rancher-monitoring",
			wantAnnotations: []string{},
			wantHIP15: []string{
				unlisted("alertmanager.alertmanagerSpec.image", "rancher/appco-alertmanager:0.30.0-12.16"),
				unlisted("crds.upgradeJob.image.busybox", "rancher/mirrored-library-busybox:1.37.0"),
				unlisted("crds.upgradeJob.image.kubectl", "rancher/kuberlr-kubectl:v8.1.2"),
				unlisted("grafana.proxy.image", "rancher/mirrored-library-nginx:1.31.3-alpine"),
				unlisted("prometheus.prometheusSpec.image", "rancher/prom-prometheus:v3.8.1"),
				unlisted("prometheusOperator.admissionWebhooks.deployment.image", "rancher/mirrored-prometheus-operator-admission-webhook:v0.87.1"),
				unlisted("prometheusOperator.admissionWebhooks.patch.image", "rancher/mirrored-jkroepke-kube-webhook-certgen:1.7.4"),
				unlisted("prometheusOperator.image", "rancher/mirrored-prometheus-operator-prometheus-operator:v0.87.1"),
				unlisted("prometheusOperator.prometheusConfigReloader.image", "rancher/mirrored-prometheus-operator-prometheus-config-reloader:v0.87.1"),
				unlisted("prometheusOperator.thanosImage", "rancher/appco-thanos:0.40.1-8.1"),
			},
		},
		{
			name:            "rancher-monitoring-crd has 1 image that is not listed",
			dataDir:         "rancher-monitoring-crd",
			chart:           "rancher-monitoring-crd",
			kind:            "crd",
			family:          "rancher-monitoring",
			wantAnnotations: []string{},
			wantHIP15: []string{
				unlisted("image", "rancher/kuberlr-kubectl:v8.1.2"),
			},
		},
		{
			name:    "longhorn has a kube-version without an upper bound and 13 images that are not listed",
			dataDir: "longhorn",
			chart:   "longhorn",
			kind:    "main",
			family:  "longhorn",
			wantAnnotations: []string{
				`catalog.cattle.io/kube-version ">= 1.25.0-0": must look like ">= 1.34.0-0 < 1.37.0-0"`,
			},
			wantHIP15: []string{
				unlisted("image.csi.attacher", "rancher/mirrored-longhornio-csi-attacher:v4.12.0"),
				unlisted("image.csi.livenessProbe", "rancher/mirrored-longhornio-livenessprobe:v2.19.0"),
				unlisted("image.csi.nodeDriverRegistrar", "rancher/mirrored-longhornio-csi-node-driver-registrar:v2.17.0"),
				unlisted("image.csi.provisioner", "rancher/mirrored-longhornio-csi-provisioner:v5.3.0"),
				unlisted("image.csi.resizer", "rancher/mirrored-longhornio-csi-resizer:v2.2.1"),
				unlisted("image.csi.snapshotter", "rancher/mirrored-longhornio-csi-snapshotter:v8.6.0"),
				unlisted("image.longhorn.backingImageManager", "rancher/mirrored-longhornio-backing-image-manager:v1.12.1"),
				unlisted("image.longhorn.engine", "rancher/mirrored-longhornio-longhorn-engine:v1.12.1"),
				unlisted("image.longhorn.instanceManager", "rancher/mirrored-longhornio-longhorn-instance-manager:v1.12.1"),
				unlisted("image.longhorn.manager", "rancher/mirrored-longhornio-longhorn-manager:v1.12.1"),
				unlisted("image.longhorn.shareManager", "rancher/mirrored-longhornio-longhorn-share-manager:v1.12.1"),
				unlisted("image.longhorn.supportBundleKit", "rancher/mirrored-longhornio-support-bundle-kit:v0.0.92"),
				unlisted("image.longhorn.ui", "rancher/mirrored-longhornio-longhorn-ui:v1.12.1"),
			},
		},
		{
			name:    "rancher-webhook has a kube-version without a lower bound and 1 image that is not listed",
			dataDir: "rancher-webhook",
			chart:   "rancher-webhook",
			kind:    "main",
			family:  "rancher-webhook",
			wantAnnotations: []string{
				`catalog.cattle.io/kube-version "< 1.37.0-0": must look like ">= 1.34.0-0 < 1.37.0-0"`,
			},
			wantHIP15: []string{
				unlisted("image", "rancher/rancher-webhook:v0.11.4"),
			},
		},
		{
			name:            "system-upgrade-controller has 2 images that are not listed",
			dataDir:         "system-upgrade-controller",
			chart:           "system-upgrade-controller",
			kind:            "main",
			family:          "system-upgrade-controller",
			wantAnnotations: []string{},
			wantHIP15: []string{
				unlisted("kubectl.image", "rancher/kuberlr-kubectl:v8.1.1"),
				unlisted("systemUpgradeController.image", "rancher/system-upgrade-controller:v0.20.1"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, res := checkJSON(t, harness.ChartSource(t, tt.dataDir, tt.chart))

			require.Equal(t, 1, res.ExitCode)
			require.Contains(t, res.Stderr, "chart is not valid")
			require.Equal(t, tt.chart, got.Chart)
			require.Equal(t, tt.kind, got.Kind)
			require.Equal(t, tt.family, got.Family)
			require.False(t, got.Valid)
			require.Equal(t, tt.wantAnnotations, got.Annotations.Errors)
			require.Equal(t, len(tt.wantAnnotations) == 0, got.Annotations.Valid)
			require.Equal(t, tt.wantHIP15, got.HIP15Annotations.Errors)
			require.False(t, got.HIP15Annotations.Valid)
		})
	}
}

func TestTextOutput(t *testing.T) {
	t.Run("valid chart", func(t *testing.T) {
		res := harness.Cargueiro(t, t.TempDir(), "check", "chart", harness.FleetSource(t, "fleet"))

		require.Zero(t, res.ExitCode, res.Stderr)
		require.True(t, strings.HasPrefix(res.Stdout, `chart: fleet
kind: main
family: fleet
valid: true
metadata:
  name: fleet
  version: 110.0.3+up0.16.3
`), res.Stdout)
		require.Contains(t, res.Stdout, "  kubeVersion: >= 1.34.0-0 < 1.37.0-0\n")
		require.Contains(t, res.Stdout, "annotations: valid\nhip15-annotations: valid\n")
	})

	t.Run("invalid chart", func(t *testing.T) {
		res := harness.Cargueiro(t, t.TempDir(), "check", "chart", harness.ChartSource(t, "system-upgrade-controller", "system-upgrade-controller"))

		require.Equal(t, 1, res.ExitCode)
		require.Contains(t, res.Stderr, "chart is not valid")
		require.Contains(t, res.Stdout, "valid: false\n")
		require.Contains(t, res.Stdout, "annotations: valid\nhip15-annotations: invalid\n")
		require.Contains(t, res.Stdout, "  - "+unlisted("kubectl.image", "rancher/kuberlr-kubectl:v8.1.1")+"\n")
		require.Contains(t, res.Stdout, "  - "+unlisted("systemUpgradeController.image", "rancher/system-upgrade-controller:v0.20.1")+"\n")
	})
}

// TestFailuresPrintNothing checks the cases where no result can be built: the
// error goes to stderr, stdout stays empty, and the exit code is 1.
func TestFailuresPrintNothing(t *testing.T) {
	// writeChart writes a chart directory with the files that are not empty.
	writeChart := func(t *testing.T, chartYAML, valuesYAML string) string {
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
				return []string{writeChart(t, "name: fleet\n", "")}
			},
			wantErr: "values.yaml not found",
		},
		{
			name: "Chart.yaml is not yaml",
			args: func(t *testing.T) []string {
				return []string{writeChart(t, "name: [\n", "a: 1\n")}
			},
			wantErr: "parse",
		},
		{
			name: "chart is not listed in the rules",
			args: func(t *testing.T) []string {
				return []string{writeChart(t, "name: epinio\n", "a: 1\n")}
			},
			wantErr: `chart "epinio" is not listed in the rules file`,
		},
		{
			name: "unknown output format",
			args: func(t *testing.T) []string {
				return []string{harness.FleetSource(t, "fleet"), "--output", "yaml"}
			},
			wantErr: `unknown output format "yaml"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := harness.Cargueiro(t, t.TempDir(), append([]string{"check", "chart"}, tt.args(t)...)...)

			require.Equal(t, 1, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
		})
	}
}
