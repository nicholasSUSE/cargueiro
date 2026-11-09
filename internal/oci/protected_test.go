package oci

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestGuard is the policy of the rules file: charts per edition, images,
// and two listed charts.
func newTestGuard(t *testing.T) *Guard {
	t.Helper()

	guard, err := NewGuard(Policy{
		Images: []string{"stgregistry.suse.com/rancher", "registry.suse.com/rancher"},
		Charts: map[string][]string{
			"rancher-charts": {"stgregistry.suse.com/rancher/catalog", "registry.suse.com/rancher/catalog"},
			"prime-charts":   {"stgregistry.suse.com/rancher/charts", "registry.suse.com/rancher/charts"},
		},
		Editions: map[string]string{
			"fleet":                "rancher-charts",
			"rancher-ai-agent":     "prime-charts",
			"rancher-ali-operator": "prime-charts",
		},
	})
	require.NoError(t, err)
	return guard
}

func TestNewGuardRejects(t *testing.T) {
	valid := Policy{
		Images: []string{"s.io/rancher"},
		Charts: map[string][]string{"rancher-charts": {"s.io/rancher/catalog"}},
	}
	tests := []struct {
		name    string
		mutate  func(p *Policy)
		wantErr string
	}{
		{
			name:    "no images",
			mutate:  func(p *Policy) { p.Images = nil },
			wantErr: "no image or chart repositories",
		},
		{
			name:    "no charts",
			mutate:  func(p *Policy) { p.Charts = nil },
			wantErr: "no image or chart repositories",
		},
		{
			name:    "an image repository without a path",
			mutate:  func(p *Policy) { p.Images = []string{"s.io"} },
			wantErr: `repository "s.io" has no path`,
		},
		{
			name:    "an invalid chart repository",
			mutate:  func(p *Policy) { p.Charts = map[string][]string{"rancher-charts": {"s.io/../x"}} },
			wantErr: "guard: invalid registry",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid
			tt.mutate(&p)

			guard, err := NewGuard(p)
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, guard)
		})
	}
}

func TestGuardCheck(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		chart    string
		wantErr  string
	}{
		// A listed chart goes to the repositories of its edition.
		{
			name:     "rancher chart, staging catalog",
			registry: "stgregistry.suse.com/rancher/catalog",
			chart:    "fleet",
		},
		{
			name:     "rancher chart, production catalog",
			registry: "registry.suse.com/rancher/catalog",
			chart:    "fleet",
		},
		{
			name:     "prime chart, staging charts",
			registry: "stgregistry.suse.com/rancher/charts",
			chart:    "rancher-ai-agent",
		},
		{
			name:     "prime chart, production charts",
			registry: "registry.suse.com/rancher/charts",
			chart:    "rancher-ali-operator",
		},
		{
			name:     "host in capitals, port, trailing dot and scheme",
			registry: "https://REGISTRY.SUSE.COM.:443/Rancher/Catalog/",
			chart:    "fleet",
		},

		// ... and nowhere else on the SUSE registries.
		{
			name:     "rancher chart in the prime repository",
			registry: "stgregistry.suse.com/rancher/charts",
			chart:    "fleet",
			wantErr:  "fleet is a rancher-charts chart and cannot go to stgregistry.suse.com/rancher/charts, which is for prime-charts charts",
		},
		{
			name:     "prime chart in the rancher repository",
			registry: "registry.suse.com/rancher/catalog",
			chart:    "rancher-ai-agent",
			wantErr:  "rancher-ai-agent is a prime-charts chart and cannot go to registry.suse.com/rancher/catalog, which is for rancher-charts charts",
		},
		{
			name:     "mixed case does not get around the edition",
			registry: "registry.suse.com/Rancher/CATALOG",
			chart:    "rancher-ai-agent",
			wantErr:  "rancher-ai-agent is a prime-charts chart and cannot go to registry.suse.com/Rancher/CATALOG",
		},
		{
			name:     "chart under the images path",
			registry: "registry.suse.com/rancher",
			chart:    "fleet",
			wantErr:  "fleet is a rancher-charts chart and can only go to registry.suse.com/rancher/catalog",
		},
		{
			name:     "chart below its repository",
			registry: "registry.suse.com/rancher/catalog/team",
			chart:    "fleet",
			wantErr:  "fleet is a rancher-charts chart and can only go to",
		},

		// An image dependency goes to the images path.
		{
			name:     "image, staging",
			registry: "stgregistry.suse.com/rancher",
			chart:    "mirrored-pause",
		},
		{
			name:     "image, production",
			registry: "registry.suse.com/rancher",
			chart:    "mirrored-pause",
		},
		{
			name:     "image in the rancher catalog",
			registry: "stgregistry.suse.com/rancher/catalog",
			chart:    "mirrored-pause",
			wantErr:  "mirrored-pause is not a chart listed in the rules file, so it is an image and can only go to stgregistry.suse.com/rancher",
		},
		{
			name:     "image in the prime repository",
			registry: "registry.suse.com/rancher/charts",
			chart:    "mirrored-pause",
			wantErr:  "mirrored-pause is not a chart listed in the rules file",
		},
		{
			name:     "image below the images path",
			registry: "registry.suse.com/rancher/mirrored",
			chart:    "pause",
			wantErr:  "pause is not a chart listed in the rules file",
		},
		{
			name:     "image named like a chart repository",
			registry: "registry.suse.com/rancher",
			chart:    "catalog",
			wantErr:  "catalog is not a chart listed in the rules file, so it cannot go to the chart repository registry.suse.com/rancher/catalog",
		},
		{
			name:     "image named like the prime repository",
			registry: "stgregistry.suse.com/Rancher",
			chart:    "charts",
			wantErr:  "charts is not a chart listed in the rules file, so it cannot go to the chart repository stgregistry.suse.com/rancher/charts",
		},
		{
			name:     "image on the host root",
			registry: "registry.suse.com",
			chart:    "pause",
			wantErr:  "pause is not a chart listed in the rules file",
		},

		// Any other host is not guarded.
		{
			name:     "local registry, rancher chart in the prime path",
			registry: "localhost:5051/rancher/charts",
			chart:    "fleet",
		},
		{
			name:     "local registry, image in the catalog",
			registry: "127.0.0.1:5050/rancher/catalog",
			chart:    "mirrored-pause",
		},
		{
			name:     "another SUSE host",
			registry: "registry.opensuse.org/rancher/catalog",
			chart:    "rancher-ai-agent",
		},
		{
			name:     "host that only contains the name",
			registry: "registry.suse.com.example.org/rancher/catalog",
			chart:    "rancher-ai-agent",
		},
		{
			name:     "host with the name as a suffix",
			registry: "myregistry.suse.com/rancher/charts",
			chart:    "fleet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ParseTarget(tt.registry)
			require.NoError(t, err)

			err = newTestGuard(t).Check(target, tt.chart)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, "pushing to this registry is not allowed: "+tt.wantErr)
		})
	}
}

func TestGuardGuarded(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		want     bool
	}{
		{name: "staging", registry: "stgregistry.suse.com/anything", want: true},
		{name: "production, port and capitals", registry: "REGISTRY.suse.com:443", want: true},
		{name: "local", registry: "localhost:5050", want: false},
		{name: "similar host", registry: "registry.suse.com.example.org", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ParseTarget(tt.registry)
			require.NoError(t, err)

			require.Equal(t, tt.want, newTestGuard(t).Guarded(target))
		})
	}
}

func TestPusherRefusesWhatTheGuardForbids(t *testing.T) {
	tests := []struct {
		name    string
		target  Target
		chart   string
		wantErr string
	}{
		{
			name:    "rancher chart in the prime repository",
			target:  Target{Host: "registry.suse.com", Path: "rancher/charts"},
			chart:   "fleet",
			wantErr: "fleet is a rancher-charts chart and cannot go to registry.suse.com/rancher/charts",
		},
		{
			name:    "image in the catalog, with case and port tricks",
			target:  Target{Host: "Registry.SUSE.com.:443", Path: "Rancher/CATALOG"},
			chart:   "pause",
			wantErr: "pause is not a chart listed in the rules file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive := writeChart(t, tt.chart, "1.0.0")
			fake := &fakeClient{}

			got, err := NewPusher(fake, tt.target, newTestGuard(t)).Push(context.Background(), archive)

			require.ErrorContains(t, err, "pushing to this registry is not allowed: "+tt.wantErr)
			require.Empty(t, got)
			require.Empty(t, fake.refs, "nothing may reach the registry")
		})
	}
}

func TestPusherPushesWhatTheGuardAllows(t *testing.T) {
	tests := []struct {
		name    string
		target  Target
		chart   string
		wantRef string
	}{
		{
			name:    "rancher chart in the catalog",
			target:  Target{Host: "stgregistry.suse.com", Path: "rancher/catalog"},
			chart:   "fleet",
			wantRef: "stgregistry.suse.com/rancher/catalog/fleet:1.0.0",
		},
		{
			name:    "image in the images path",
			target:  Target{Host: "registry.suse.com", Path: "rancher"},
			chart:   "pause",
			wantRef: "registry.suse.com/rancher/pause:1.0.0",
		},
		{
			name:    "a local registry, the same chart again",
			target:  Target{Host: "localhost:5050", Path: "rancher/charts"},
			chart:   "fleet",
			wantRef: "localhost:5050/rancher/charts/fleet:1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive := writeChart(t, tt.chart, "1.0.0")
			fake := &fakeClient{}

			_, err := NewPusher(fake, tt.target, newTestGuard(t)).Push(context.Background(), archive)

			require.NoError(t, err)
			require.Equal(t, []string{tt.wantRef}, fake.refs)
		})
	}
}

func TestReadIdentity(t *testing.T) {
	name, version, err := ReadIdentity(writeChart(t, "fleet", "1.2.3"))
	require.NoError(t, err)
	require.Equal(t, "fleet", name)
	require.Equal(t, "1.2.3", version)

	_, _, err = ReadIdentity("/no/such/archive.tgz")
	require.ErrorContains(t, err, "read archive /no/such/archive.tgz")
}
