package chart

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	ci "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
)

func TestPackage(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr error
		wantMsg string
	}{
		{name: "valid chart", src: "testdata/valid"},
		{name: "missing directory", src: "testdata/absent", wantErr: ErrNoChartYaml},
		{name: "directory without Chart.yaml", src: "testdata/noyaml", wantErr: ErrNoChartYaml},
		{name: "invalid version", src: "testdata/badversion", wantMsg: "package chart"},
		{name: "declared dependency not vendored", src: "testdata/missingdeps", wantMsg: "package chart"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "out")

			got, err := Package(context.Background(), tt.src, dest)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			if tt.wantMsg != "" {
				require.ErrorContains(t, err, tt.wantMsg)
				return
			}
			require.NoError(t, err)

			require.True(t, filepath.IsAbs(got))
			require.Equal(t, filepath.Join(dest, "valid-0.1.0.tgz"), got)

			loaded, err := loader.Load(got)
			require.NoError(t, err)
			acc, err := ci.NewAccessor(loaded)
			require.NoError(t, err)
			require.Equal(t, "valid", acc.Name())
			require.Equal(t, "0.1.0", acc.MetadataAsMap()["Version"])
		})
	}
}

func TestPackageCreatesDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "a", "b")

	_, err := Package(context.Background(), "testdata/valid", dest)
	require.NoError(t, err)

	info, err := os.Stat(dest)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestPackageCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Package(ctx, "testdata/valid", t.TempDir())
	require.ErrorIs(t, err, context.Canceled)
}
