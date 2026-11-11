//go:build e2e

package shipping

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

func TestShipThenIndexReadWithCredentials(t *testing.T) {
	logo := newLogoServer(t, http.StatusOK)
	address := harness.HTTPRegistry(t)
	_, path, reference := pushComplianceTo(t, address, logo.URL+"/compliance")
	right := []string{"CARGUEIRO_USERNAME=" + harness.RegistryUser, "CARGUEIRO_PASSWORD=" + harness.RegistryPassword}
	wrong := []string{"CARGUEIRO_USERNAME=" + harness.RegistryUser, "CARGUEIRO_PASSWORD=not-the-password"}

	t.Run("ship", func(t *testing.T) {
		tests := []struct {
			name        string
			env         []string
			wantOK      bool
			wantStderr  string
			notInStderr string
		}{
			{
				name:   "the right credentials",
				env:    right,
				wantOK: true,
			},
			{
				name:        "the wrong password",
				env:         wrong,
				wantStderr:  "refused the credentials",
				notInStderr: "not-the-password",
			},
			{
				name:       "none",
				wantStderr: "granted no access",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo := charts(t)

				res := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: append(harness.GitEnv(), tt.env...)},
					"ship", reference, "--repo", repo, "--plain-http")

				if tt.wantOK {
					require.Zero(t, res.ExitCode, res.Stderr)
					require.FileExists(t, filepath.Join(repo, "entries", chartName, chartVersion+".yaml"))
					return
				}
				require.NotZero(t, res.ExitCode)
				require.Contains(t, res.Stderr, tt.wantStderr)
				if tt.notInStderr != "" {
					require.NotContains(t, res.Stderr, tt.notInStderr)
				}
				requireClean(t, repo)
			})
		}
	})

	t.Run("index", func(t *testing.T) {
		repo := charts(t)
		shipped := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: append(harness.GitEnv(), right...)},
			"ship", reference, "--repo", repo, "--plain-http")
		require.Zero(t, shipped.ExitCode, shipped.Stderr)
		registry := address + "/" + path
		archive := filepath.Join(repo, "assets", chartName, chartName+"-"+chartVersion+".tgz")

		withoutCredentials := harness.Cargueiro(t, t.TempDir(), "index", "--repo", repo, "--registry", registry, "--plain-http")
		require.NotZero(t, withoutCredentials.ExitCode)
		require.Contains(t, withoutCredentials.Stderr, "granted no access")
		require.NoFileExists(t, archive, "nothing is written when a pull fails")

		wrongPassword := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: wrong},
			"index", "--repo", repo, "--registry", registry, "--plain-http")
		require.NotZero(t, wrongPassword.ExitCode)
		require.Contains(t, wrongPassword.Stderr, "refused the credentials")
		require.NotContains(t, wrongPassword.Stderr, "not-the-password")
		require.NoFileExists(t, archive)

		rightPassword := harness.CargueiroWith(t, harness.Options{Dir: t.TempDir(), Env: right},
			"index", "--repo", repo, "--registry", registry, "--plain-http")
		require.Zero(t, rightPassword.ExitCode, rightPassword.Stderr)
		require.Equal(t, "add "+chartName+" "+chartVersion+"\n", rightPassword.Stdout)
		require.FileExists(t, archive)
	})
}
