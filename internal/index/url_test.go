package index

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

const (
	testRepository = "stgregistry.suse.com/rancher/catalog/fleet"
	testDigest     = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func TestCheckURL(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		version    string
		urls       []string
		wantErr    string
	}{
		{
			name:       "the url of the version with its digest",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://" + testRepository + ":111.0.3_up0.17.3@" + testDigest},
		},
		{
			name:       "a plain version",
			repository: testRepository,
			version:    "111.0.3",
			urls:       []string{"oci://" + testRepository + ":111.0.3@" + testDigest},
		},
		{
			name:       "a prerelease",
			repository: testRepository,
			version:    "111.0.4+up0.17.4-rc.1",
			urls:       []string{"oci://" + testRepository + ":111.0.4_up0.17.4-rc.1@" + testDigest},
		},
		{
			name:       "no url",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       nil,
			wantErr:    "fleet 111.0.3+up0.17.3 has 0 urls: it must have exactly one",
		},
		{
			name:       "two urls",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls: []string{
				"oci://" + testRepository + ":111.0.3_up0.17.3@" + testDigest,
				"oci://" + testRepository + ":111.0.3_up0.17.3@" + testDigest,
			},
			wantErr: "fleet 111.0.3+up0.17.3 has 2 urls: it must have exactly one",
		},
		{
			name:       "no digest",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://" + testRepository + ":111.0.3_up0.17.3"},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "the version as it is, with its +, in the tag",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://" + testRepository + ":111.0.3+up0.17.3@" + testDigest},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "another version",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://" + testRepository + ":111.0.4_up0.17.4@" + testDigest},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "another registry",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://registry.suse.com/rancher/catalog/fleet:111.0.3_up0.17.3@" + testDigest},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "another chart",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://stgregistry.suse.com/rancher/catalog/fleet-crd:111.0.3_up0.17.3@" + testDigest},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "no scheme",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{testRepository + ":111.0.3_up0.17.3@" + testDigest},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "a digest that is too short",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://" + testRepository + ":111.0.3_up0.17.3@sha256:0123"},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "a digest in capitals",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://" + testRepository + ":111.0.3_up0.17.3@" + strings.ToUpper(testDigest)},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "another algorithm",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://" + testRepository + ":111.0.3_up0.17.3@sha512:" + strings.Repeat("0", 128)},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
		{
			name:       "something after the digest",
			repository: testRepository,
			version:    "111.0.3+up0.17.3",
			urls:       []string{"oci://" + testRepository + ":111.0.3_up0.17.3@" + testDigest + "/extra"},
			wantErr:    "must be oci://" + testRepository + ":111.0.3_up0.17.3@sha256:<digest>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &repo.ChartVersion{
				Metadata: &chart.Metadata{Name: "fleet", Version: tt.version},
				URLs:     tt.urls,
			}

			err := CheckURL(tt.repository, entry)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
