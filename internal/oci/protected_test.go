package oci

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckTarget(t *testing.T) {
	tests := []struct {
		name      string
		registry  string
		protected bool
	}{
		// Protected.
		{name: "production catalog", registry: "registry.suse.com/rancher/catalog", protected: true},
		{name: "staging catalog", registry: "stgregistry.suse.com/rancher/catalog", protected: true},
		{name: "chart path under the catalog", registry: "registry.suse.com/rancher/catalog/fleet", protected: true},
		{name: "deeper path under the catalog", registry: "stgregistry.suse.com/rancher/catalog/team/charts", protected: true},
		{name: "uppercase host", registry: "REGISTRY.SUSE.COM/rancher/catalog", protected: true},
		{name: "mixed-case path", registry: "registry.suse.com/Rancher/Catalog", protected: true},
		{name: "trailing dot in host", registry: "registry.suse.com./rancher/catalog", protected: true},
		{name: "default HTTPS port", registry: "registry.suse.com:443/rancher/catalog", protected: true},
		{name: "any other port", registry: "registry.suse.com:5000/rancher/catalog", protected: true},
		{name: "oci scheme", registry: "oci://registry.suse.com/rancher/catalog", protected: true},
		{name: "https scheme", registry: "https://stgregistry.suse.com/rancher/catalog", protected: true},
		{name: "trailing slashes", registry: "registry.suse.com/rancher/catalog///", protected: true},
		{name: "surrounding space", registry: "  registry.suse.com/rancher/catalog  ", protected: true},

		// Allowed.
		{name: "local registry, same path", registry: "localhost:5051/rancher/catalog"},
		{name: "loopback, same path", registry: "127.0.0.1:5050/rancher/catalog"},
		{name: "protected host without a path", registry: "registry.suse.com"},
		{name: "protected host, parent path", registry: "registry.suse.com/rancher"},
		{name: "protected host, other path", registry: "registry.suse.com/rancher/charts"},
		{name: "similar name", registry: "registry.suse.com/rancher/catalogue"},
		{name: "similar name with a prefix", registry: "registry.suse.com/rancher/catalog-old"},
		{name: "catalog is not at the root", registry: "registry.suse.com/other/rancher/catalog"},
		{name: "another SUSE host", registry: "registry.opensuse.org/rancher/catalog"},
		{name: "host that only contains the name", registry: "registry.suse.com.example.org/rancher/catalog"},
		{name: "host with the name as a suffix", registry: "myregistry.suse.com/rancher/catalog"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ParseTarget(tt.registry)
			require.NoError(t, err)

			err = CheckTarget(target)

			if tt.protected {
				require.ErrorIs(t, err, ErrProtectedRegistry)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCheckTargetErrorNamesTheRegistry(t *testing.T) {
	err := CheckTarget(Target{Host: "registry.suse.com", Path: "rancher/catalog"})

	require.ErrorContains(t, err, "registry.suse.com/rancher/catalog")
	require.ErrorContains(t, err, "never be overwritten")
}

func TestPusherRefusesProtectedTargets(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		chart  string
	}{
		{name: "production catalog", target: Target{Host: "registry.suse.com", Path: "rancher/catalog"}, chart: "fleet"},
		{name: "staging catalog", target: Target{Host: "stgregistry.suse.com", Path: "rancher/catalog"}, chart: "fleet"},
		{name: "case and port tricks", target: Target{Host: "Registry.SUSE.com.:443", Path: "Rancher/CATALOG"}, chart: "fleet"},
		{name: "the chart name completes the path", target: Target{Host: "registry.suse.com", Path: "rancher"}, chart: "catalog"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive := writeChart(t, tt.chart, "1.0.0")
			fake := &fakeClient{}

			got, err := NewPusher(fake, tt.target).Push(context.Background(), archive)

			require.ErrorIs(t, err, ErrProtectedRegistry)
			require.Empty(t, got)
			require.Empty(t, fake.refs, "nothing may reach the registry")
		})
	}
}

func TestPusherRefusesProtectedTargetBeforeReadingTheArchive(t *testing.T) {
	fake := &fakeClient{}
	target := Target{Host: "registry.suse.com", Path: "rancher/catalog"}

	_, err := NewPusher(fake, target).Push(context.Background(), "/no/such/archive.tgz")

	require.ErrorIs(t, err, ErrProtectedRegistry, "the guard must come before any other work")
}

func TestPusherAllowsOverwritingOutsideTheProtectedRegistries(t *testing.T) {
	archive := writeChart(t, "fleet", "1.0.0")
	fake := &fakeClient{}
	pusher := NewPusher(fake, Target{Host: "localhost:5050", Path: "rancher/catalog"})

	for range 2 {
		_, err := pusher.Push(context.Background(), archive)
		require.NoError(t, err)
	}

	require.Equal(t, []string{"localhost:5050/rancher/catalog/fleet:1.0.0", "localhost:5050/rancher/catalog/fleet:1.0.0"}, fake.refs,
		"the same chart may be pushed again to a local registry")
}
