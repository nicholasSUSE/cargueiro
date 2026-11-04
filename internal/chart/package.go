// Package chart packages Helm charts into distributable archives.
package chart

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v4/pkg/action"
)

// ErrNoChartYaml is returned when the source directory has no Chart.yaml.
var ErrNoChartYaml = errors.New("no Chart.yaml found")

// Package packages the chart in srcDir into destDir and returns the absolute
// path of the resulting .tgz archive. destDir is created if it does not exist.
//
// Dependencies declared in Chart.yaml must already be present in srcDir/charts;
// Package never fetches anything.
func Package(ctx context.Context, srcDir, destDir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if _, err := os.Stat(filepath.Join(srcDir, "Chart.yaml")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w in %s", ErrNoChartYaml, srcDir)
		}
		return "", fmt.Errorf("stat chart %s: %w", srcDir, err)
	}

	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return "", fmt.Errorf("resolve destination %s: %w", destDir, err)
	}
	if err := os.MkdirAll(absDest, 0o755); err != nil {
		return "", fmt.Errorf("create destination %s: %w", absDest, err)
	}

	packager := action.NewPackage()
	packager.Destination = absDest

	archive, err := packager.Run(srcDir, nil)
	if err != nil {
		return "", fmt.Errorf("package chart %s: %w", srcDir, err)
	}

	return archive, nil
}
