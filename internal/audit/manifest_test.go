package audit

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/oci"
)

const fleetTag = "stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3"

func TestCheckManifests(t *testing.T) {
	tests := []struct {
		name    string
		pinned  string
		stored  map[string]oci.ManifestInfo
		failure error
		wantErr []string
	}{
		{
			name:   "the tag names the manifest the url pins",
			pinned: manifestA,
			stored: map[string]oci.ManifestInfo{fleetTag: {ManifestDigest: manifestA}},
		},
		{
			name:    "the tag was pushed again with other content",
			pinned:  manifestA,
			stored:  map[string]oci.ManifestInfo{fleetTag: {ManifestDigest: manifestB}},
			wantErr: []string{"fleet 111.0.3+up0.17.3: the tag 111.0.3_up0.17.3 of stgregistry.suse.com/rancher/catalog/fleet is the manifest " + manifestB + " in the registry, but the url pins " + manifestA},
		},
		{
			name:    "the version was never pushed",
			pinned:  manifestA,
			stored:  map[string]oci.ManifestInfo{},
			wantErr: []string{"fleet 111.0.3+up0.17.3: ", "was not found in the registry"},
		},
		{
			name:    "the registry cannot be reached",
			pinned:  manifestA,
			failure: errors.New("connection refused"),
			wantErr: []string{"fleet 111.0.3+up0.17.3: connection refused"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &registryStub{tags: tt.stored, failWith: tt.failure}
			dir := writeIndex(t, fleetIndex(fleetVersion("111.0.3+up0.17.3", tt.pinned, archiveC)))

			result := run(t, Options{Dir: dir, Fetch: stub.fetch})[1]

			require.Equal(t, "manifest", result.Check)
			if len(tt.wantErr) == 0 {
				require.True(t, result.Valid, result.Error)
				require.Equal(t, NoError, result.Error)
				return
			}
			require.False(t, result.Valid)
			for _, want := range tt.wantErr {
				require.Contains(t, result.Error, want)
			}
		})
	}
}

func TestCheckManifestsWithoutAPinnedDigest(t *testing.T) {
	text := "apiVersion: v1\nentries:\n  fleet:\n  - apiVersion: v2\n    name: fleet\n    version: 111.0.3+up0.17.3\n    digest: " + archiveC +
		"\n    urls:\n    - oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3\n"
	stub := &registryStub{tags: map[string]oci.ManifestInfo{fleetTag: {ManifestDigest: manifestA}}}

	result := run(t, Options{Dir: writeIndex(t, text), Fetch: stub.fetch})[1]

	require.False(t, result.Valid)
	require.Contains(t, result.Error, "fleet 111.0.3+up0.17.3: its url pins no digest to compare with")
}

func TestCheckManifestsNamesEveryVersionAtFault(t *testing.T) {
	stub := &registryStub{tags: map[string]oci.ManifestInfo{
		"stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3": {ManifestDigest: manifestB},
		"stgregistry.suse.com/rancher/catalog/fleet:111.0.4_up0.17.4": {ManifestDigest: manifestA},
	}}
	dir := writeIndex(t, fleetIndex(
		fleetVersion("111.0.3+up0.17.3", manifestA, archiveC),
		fleetVersion("111.0.4+up0.17.4", manifestA, archiveC),
		fleetVersion("111.0.5+up0.17.5", manifestA, archiveC),
	))

	result := run(t, Options{Dir: dir, Fetch: stub.fetch})[1]

	require.False(t, result.Valid)
	require.Contains(t, result.Error, "fleet 111.0.3+up0.17.3")
	require.Contains(t, result.Error, "fleet 111.0.5+up0.17.5")
	require.NotContains(t, result.Error, "fleet 111.0.4+up0.17.4")
	require.Contains(t, result.Description, "3 versions")
}
