package audit

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/oci"
)

func TestCheckArchives(t *testing.T) {
	tests := []struct {
		name    string
		stored  map[string]oci.ManifestInfo
		failure error
		wantErr string
	}{
		{
			name:   "the digest of the index is the one of the chart in the registry",
			stored: map[string]oci.ManifestInfo{fleetTag: {ManifestDigest: manifestA, ChartDigest: "sha256:" + archiveC}},
		},
		{
			name:    "the chart in the registry is another archive",
			stored:  map[string]oci.ManifestInfo{fleetTag: {ManifestDigest: manifestA, ChartDigest: "sha256:" + archiveD}},
			wantErr: "fleet 111.0.3+up0.17.3: the index has the digest sha256:" + archiveC + ", but the chart in the registry has sha256:" + archiveD,
		},
		{
			name:   "a version the registry does not have is left to the manifest check",
			stored: map[string]oci.ManifestInfo{},
		},
		{
			name:    "a registry that cannot be reached is left to the manifest check",
			failure: errors.New("connection refused"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &registryStub{tags: tt.stored, failWith: tt.failure}
			dir := writeIndex(t, fleetIndex(fleetVersion("111.0.3+up0.17.3", manifestA, archiveC)))

			result := run(t, Options{Dir: dir, Fetch: stub.fetch})[2]

			require.Equal(t, "archive", result.Check)
			if tt.wantErr == "" {
				require.True(t, result.Valid, result.Error)
				require.Equal(t, NoError, result.Error)
				return
			}
			require.False(t, result.Valid)
			require.Contains(t, result.Error, tt.wantErr)
		})
	}
}

func TestCheckArchivesNamesEveryVersionAtFault(t *testing.T) {
	right := oci.ManifestInfo{ManifestDigest: manifestA, ChartDigest: "sha256:" + archiveC}
	wrong := oci.ManifestInfo{ManifestDigest: manifestA, ChartDigest: "sha256:" + archiveD}
	stub := &registryStub{tags: map[string]oci.ManifestInfo{
		"stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3": wrong,
		"stgregistry.suse.com/rancher/catalog/fleet:111.0.4_up0.17.4": right,
		"stgregistry.suse.com/rancher/catalog/fleet:111.0.5_up0.17.5": wrong,
	}}
	dir := writeIndex(t, fleetIndex(
		fleetVersion("111.0.3+up0.17.3", manifestA, archiveC),
		fleetVersion("111.0.4+up0.17.4", manifestA, archiveC),
		fleetVersion("111.0.5+up0.17.5", manifestA, archiveC),
	))

	result := run(t, Options{Dir: dir, Fetch: stub.fetch})[2]

	require.False(t, result.Valid)
	require.Contains(t, result.Error, "fleet 111.0.3+up0.17.3")
	require.Contains(t, result.Error, "fleet 111.0.5+up0.17.5")
	require.NotContains(t, result.Error, "fleet 111.0.4+up0.17.4")
}
