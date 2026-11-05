package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// defaultOutputDir is where archives are written unless --output-dir is set.
const defaultOutputDir = "assets"

// packageFunc packages the chart in srcDir into destDir and returns the
// path of the archive it wrote.
type packageFunc func(ctx context.Context, srcDir, destDir string) (string, error)

func newPackageCmd(opts *rootOptions, pkg packageFunc) (*cobra.Command, error) {
	var outputDir string

	cmd := &cobra.Command{
		Use:  "package <chart-dir>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			opts.log.DebugContext(cmd.Context(), "packaging chart", "chart", dir, "output", outputDir)

			archive, err := pkg(cmd.Context(), dir, outputDir)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), archive); err != nil {
				return fmt.Errorf("write archive path: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", defaultOutputDir, "directory to write archives to")
	if err := applyDoc(cmd, "package"); err != nil {
		return nil, err
	}

	return cmd, nil
}
