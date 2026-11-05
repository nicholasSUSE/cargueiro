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
			return "", fmt.Errorf("no Chart.yaml found in %s", srcDir)
		}
		return "", err
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
