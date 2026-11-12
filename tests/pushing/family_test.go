//go:build e2e

package pushing

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/tests/internal/harness"
)

func TestPushFamilyOfFleet(t *testing.T) {
	for _, tr := range harness.Transports() {
		t.Run(tr.Name, func(t *testing.T) {
			s := harness.NewSession(t)
			address, path := s.LoginVia(t, tr), repoPath()
			// Given in the opposite order of the family, to show that the order
			// comes from the rules.
			args := []string{"push", "--family", "fleet", "--registry", address + "/" + path}
			for _, chart := range fleetCharts() {
				args = append(args, published(t, chart))
			}

			res := s.Run(t, "", nil, args...)

			requireNoPassword(t, res)
			require.Zero(t, res.ExitCode, res.Stderr)
			want := ""
			for _, chart := range []string{"fleet", "fleet-agent", "fleet-crd"} {
				want += fmt.Sprintf("oci://%s/%s/%s:%s\n", address, path, chart, registryTag)
			}
			require.Equal(t, want, res.Stdout, "the main chart first, then the order of rules.yaml")

			for _, chart := range fleetCharts() {
				pulled, pullRes := pull(t, tr, address, path, chart)
				require.NotEmpty(t, pulled, "%s: %s", chart, pullRes.Stderr)
				require.Equal(t, harness.Manifest(t, published(t, chart)), harness.Manifest(t, pulled),
					"%s: what the registry gives back is what was pushed", chart)
			}
		})
	}
}

func TestPushFamilyPushesNothingForAnIncompleteFamily(t *testing.T) {
	tr := harness.Transports()[0]
	s := harness.NewSession(t)
	address, path := s.LoginVia(t, tr), repoPath()

	res := s.Run(t, "", nil, "push", "--family", "fleet", "--registry", address+"/"+path,
		published(t, "fleet"), published(t, "fleet-crd"))

	requireNoPassword(t, res)
	require.NotZero(t, res.ExitCode)
	require.Contains(t, res.Stderr, `the family "fleet" is missing fleet-agent`)
	require.Empty(t, res.Stdout)
	listed := s.Run(t, "", nil, "list", address+"/"+path+"/fleet", "--plain-http")
	require.NotZero(t, listed.ExitCode, "nothing was pushed: the repository does not exist")
}

func TestPushFamilyIsRefusedByTheRulesBeforeAnyConnection(t *testing.T) {
	s := harness.NewSession(t)
	args := []string{"push", "--family", "fleet", "--registry", "stgregistry.suse.com/rancher"}
	for _, chart := range fleetCharts() {
		args = append(args, published(t, chart))
	}

	// There is no network, and no login: the rules decide first.
	res := s.Run(t, "", deadProxy(), args...)

	require.Equal(t, 1, res.ExitCode)
	require.Empty(t, res.Stdout)
	require.Contains(t, res.Stderr, "pushing to this registry is not allowed")
	require.Contains(t, res.Stderr, "can only go to stgregistry.suse.com/rancher/catalog")
}
