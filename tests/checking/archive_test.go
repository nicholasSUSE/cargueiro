//go:build e2e

package checking

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/check"
	"github.com/nicholasSUSE/cargueiro/tests/internal/harness"
)

// checkArchive runs check archive on an archive.
func checkArchive(t *testing.T, args ...string) harness.Result {
	t.Helper()

	return harness.Cargueiro(t, t.TempDir(), append([]string{"check", "archive"}, args...)...)
}

func TestCheckArchiveFleetCharts(t *testing.T) {
	for _, name := range []string{"fleet-crd", "fleet", "fleet-agent"} {
		t.Run(name, func(t *testing.T) {
			archive := harness.FleetPublished(t, name, fleetVersion)

			res := checkArchive(t, archive, "--output", "json")
			require.Zero(t, res.ExitCode, res.Stderr)
			require.Empty(t, res.Stderr)
			var got check.Result
			require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got), res.Stdout)
			require.True(t, got.Valid)
			require.Equal(t, name, got.Chart)

			// The same chart gives the same report from an archive and from a directory.
			fromArchive := checkArchive(t, archive)
			fromDirectory := checkChartText(t, harness.FleetSource(t, name))
			require.Zero(t, fromArchive.ExitCode, fromArchive.Stderr)
			require.Equal(t, fromDirectory.Stdout, fromArchive.Stdout)
		})
	}
}

func TestCheckArchiveBreaksTheRules(t *testing.T) {
	for _, name := range []string{"system-upgrade-controller", "longhorn"} {
		t.Run(name, func(t *testing.T) {
			archive := fixtureArchive(t, name, name)

			fromArchive := checkArchive(t, archive)
			fromDirectory := checkChartText(t, harness.ChartSource(t, name, name))

			require.Equal(t, 1, fromArchive.ExitCode)
			require.Contains(t, fromArchive.Stderr, "chart is not valid")
			require.Contains(t, fromArchive.Stdout, "valid: false\n")
			require.Equal(t, fromDirectory.Stdout, fromArchive.Stdout)
		})
	}

	t.Run("--no-error-hip-15 for a chart that only breaks the HIP-15 rules", func(t *testing.T) {
		archive := fixtureArchive(t, "system-upgrade-controller", "system-upgrade-controller")

		res := checkArchive(t, archive, "--no-error-hip-15")

		require.Zero(t, res.ExitCode, res.Stderr)
		require.Contains(t, res.Stdout, "hip15-annotations: invalid\n")
		require.Equal(t, "chart is not valid (HIP-15 errors ignored by --no-error-hip-15)\n", res.Stderr)
	})
}

func TestCheckArchiveFailsWithoutPrinting(t *testing.T) {
	notAnArchive := filepath.Join(t.TempDir(), "not.tgz")
	require.NoError(t, os.WriteFile(notAnArchive, []byte("this is not an archive"), 0o600))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "the file does not exist",
			args:    []string{filepath.Join(t.TempDir(), "nope.tgz")},
			wantErr: "nope.tgz not found",
		},
		{
			name:    "it is not an archive",
			args:    []string{notAnArchive},
			wantErr: "the archive is not gzip",
		},
		{
			name:    "it is a directory",
			args:    []string{harness.FleetSource(t, "fleet")},
			wantErr: "the archive is not gzip",
		},
		{
			name:    "an unknown output format",
			args:    []string{harness.FleetPublished(t, "fleet", fleetVersion), "--output", "yaml"},
			wantErr: `unknown output format "yaml"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := checkArchive(t, tt.args...)

			require.Equal(t, 1, res.ExitCode)
			require.Empty(t, res.Stdout)
			require.Contains(t, res.Stderr, tt.wantErr)
		})
	}
}
