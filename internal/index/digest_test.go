package index

import (
	"testing"

	"github.com/stretchr/testify/require"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

const (
	// helloDigest is the sha256 of "hello\n".
	helloDigest = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	// emptyDigest is the sha256 of nothing.
	emptyDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func TestArchiveDigests(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"assets/fleet/fleet-1.tgz":         "hello\n",
		"assets/fleet/fleet-2.tgz":         "",
		"assets/fleet-crd/fleet-crd-1.tgz": "hello\n",
	})
	all := &repo.IndexFile{
		APIVersion: "v1",
		Entries: map[string]repo.ChartVersions{
			"fleet":     {entry("fleet", "1", "d"), entry("fleet", "2", "d"), entry("fleet", "3", "d")},
			"fleet-crd": {entry("fleet-crd", "1", "d")},
		},
	}

	got, err := archiveDigests(dir, all)

	require.NoError(t, err)
	require.Equal(t, map[Ref]string{
		{Chart: "fleet", Version: "1"}:     helloDigest,
		{Chart: "fleet", Version: "2"}:     emptyDigest,
		{Chart: "fleet-crd", Version: "1"}: helloDigest,
	}, got, "a version with no archive has no digest")
}

func TestArchiveDigestsFails(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"assets/fleet/fleet-1.tgz/inside": "x"})
	all := indexOf("fleet", entry("fleet", "1", "d"))

	got, err := archiveDigests(dir, all)

	require.ErrorContains(t, err, "digest of fleet 1")
	require.Nil(t, got)
}

func TestSetDigests(t *testing.T) {
	all := indexOf("fleet", entry("fleet", "1", "d"), entry("fleet", "2", "d"))

	setDigests(all, map[Ref]string{{Chart: "fleet", Version: "1"}: helloDigest})

	require.Equal(t, helloDigest, all.Entries["fleet"][0].Digest)
	require.Empty(t, all.Entries["fleet"][1].Digest, "a version with no digest keeps none")
}

func TestCheckArchivesUnchanged(t *testing.T) {
	published := func(digests ...string) *repo.IndexFile {
		versions := make(repo.ChartVersions, 0, len(digests))
		for i, d := range digests {
			e := entry("fleet", string(rune('1'+i)), "d")
			e.Digest = d
			versions = append(versions, e)
		}
		return indexOf("fleet", versions...)
	}

	tests := []struct {
		name      string
		published *repo.IndexFile
		digests   map[Ref]string
		wantErr   []string
	}{
		{
			name:      "nothing is published yet",
			published: nil,
			digests:   map[Ref]string{{Chart: "fleet", Version: "1"}: helloDigest},
		},
		{
			name:      "the archives are as they were published",
			published: published(helloDigest, emptyDigest),
			digests: map[Ref]string{
				{Chart: "fleet", Version: "1"}: helloDigest,
				{Chart: "fleet", Version: "2"}: emptyDigest,
			},
		},
		{
			name:      "an archive that changed",
			published: published(helloDigest),
			digests:   map[Ref]string{{Chart: "fleet", Version: "1"}: emptyDigest},
			wantErr: []string{
				"fleet 1: assets/fleet/fleet-1.tgz has changed since it was published",
				"index.yaml has the digest " + helloDigest + " and the file has " + emptyDigest,
				"A published archive is never allowed to change",
			},
		},
		{
			name:      "a published line with no digest",
			published: published(""),
			digests:   map[Ref]string{{Chart: "fleet", Version: "1"}: helloDigest},
			wantErr: []string{
				"fleet 1: assets/fleet/fleet-1.tgz has changed since it was published",
				"index.yaml has the digest  and the file has " + helloDigest,
			},
		},
		{
			name:      "every archive that changed is named",
			published: published(helloDigest, helloDigest),
			digests: map[Ref]string{
				{Chart: "fleet", Version: "1"}: emptyDigest,
				{Chart: "fleet", Version: "2"}: emptyDigest,
			},
			wantErr: []string{"fleet 1: assets/fleet/fleet-1.tgz has changed", "fleet 2: assets/fleet/fleet-2.tgz has changed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkArchivesUnchanged(tt.published, tt.digests)

			if len(tt.wantErr) == 0 {
				require.NoError(t, err)
				return
			}
			for _, msg := range tt.wantErr {
				require.ErrorContains(t, err, msg)
			}
		})
	}
}
