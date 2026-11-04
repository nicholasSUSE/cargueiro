package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// defaultOutputDir is where archives are written unless --output-dir is set.
const defaultOutputDir = "dist"

// packageFunc packages the chart in srcDir into destDir and returns the
// path of the archive it wrote.
type packageFunc func(ctx context.Context, srcDir, destDir string) (string, error)

func newPackageCmd(opts *rootOptions, pkg packageFunc) *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "package <chart-dir>...",
		Short: "Package Helm charts into .tgz archives",
		Long: `Package each chart directory into <name>-<version>.tgz inside the output directory.

Archive paths are printed to stdout, one per line, in argument order.
Logs go to stderr. Every chart is attempted; the exit code is non-zero if any fails.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, chartDirs []string) error {
			var errs []error
			for _, dir := range chartDirs {
				opts.log.DebugContext(cmd.Context(), "packaging chart", "chart", dir, "output", outputDir)

				archive, err := pkg(cmd.Context(), dir, outputDir)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), archive); err != nil {
					return fmt.Errorf("write archive path: %w", err)
				}
			}
			return errors.Join(errs...)
		},
	}
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", defaultOutputDir, "directory to write archives to")

	return cmd
}
