package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/check"
	"github.com/rancher/cargueiro/internal/integrity"
	"github.com/rancher/cargueiro/internal/rules"
)

// defaultOutputDir is where archives are written unless --output-dir is set.
const defaultOutputDir = "assets"

// packageFunc packages the chart in srcDir into destDir and returns the path
// of the archive it wrote.
type packageFunc func(ctx context.Context, srcDir, destDir string) (string, error)

func newPackageCmd(opts *rootOptions, pkg packageFunc) (*cobra.Command, error) {
	var outputDir, familyName string

	cmd := &cobra.Command{
		Use: "package <chart-dir>",
		Args: func(cmd *cobra.Command, args []string) error {
			return checkArgs(familyName)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var archives []string
			if familyName != "" {
				var err error
				if archives, err = packageFamily(cmd, opts, pkg, familyName, args, outputDir); err != nil {
					return err
				}
			} else {
				archive, err := packageChart(cmd, opts, pkg, args[0], outputDir)
				if err != nil {
					return err
				}
				archives = []string{archive}
			}
			for _, archive := range archives {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), archive); err != nil {
					return fmt.Errorf("write archive path: %w", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", defaultOutputDir, "directory to write archives to")
	cmd.Flags().StringVar(&familyName, "family", "", "package every chart of this family, with one folder for each of its charts")
	if err := applyDoc(cmd, "package"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// packageChart checks the chart in dir, packages it, and checks that the archive
// is valid and has the files of the directory. It returns the archive, which it
// removes when something is wrong with it.
func packageChart(cmd *cobra.Command, opts *rootOptions, pkg packageFunc, dir, outputDir string) (string, error) {
	ctx := cmd.Context()
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return "", err
	}

	opts.log.DebugContext(ctx, "verifying the chart", "chart", dir)
	files, err := chart.Load(dir)
	if err != nil {
		return "", err
	}
	if err := requireValid(cmd, cfg, files, "the chart is not valid, so it was not packaged"); err != nil {
		return "", err
	}
	fromDir, err := chart.DirManifest(dir)
	if err != nil {
		return "", err
	}

	opts.log.DebugContext(ctx, "packaging chart", "chart", dir, "output", outputDir)
	archive, err := pkg(ctx, dir, outputDir)
	if err != nil {
		return "", err
	}

	if err := verifyArchive(cmd, opts, cfg, dir, archive, fromDir); err != nil {
		if removeErr := os.Remove(archive); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove %s: %w", archive, removeErr))
		}
		return "", err
	}
	return archive, nil
}

// verifyArchive checks that the archive is valid, and that it has the files of
// the directory it was made from.
func verifyArchive(cmd *cobra.Command, opts *rootOptions, cfg *rules.Config, dir, archive string, fromDir *integrity.Manifest) error {
	ctx := cmd.Context()

	opts.log.DebugContext(ctx, "verifying the archive", "archive", archive)
	f, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("open %s: %w", archive, err)
	}
	defer func() { _ = f.Close() }()
	files, err := chart.LoadArchive(f)
	if err != nil {
		return err
	}
	if err := requireValid(cmd, cfg, files, "the archive is not valid, so it was removed"); err != nil {
		return err
	}

	opts.log.DebugContext(ctx, "verifying digests", "files", fromDir.Len())
	fromArchive, err := chart.ArchiveManifest(archive)
	if err != nil {
		return err
	}
	if err := integrity.Compare(fromDir, fromArchive).Err("directory", "archive"); err != nil {
		return fmt.Errorf("the archive does not have the files of the directory, so it was removed: %w", err)
	}

	left, err := chart.LeftOut(dir, fromDir)
	if err != nil {
		return err
	}
	if len(left) > 0 {
		opts.log.DebugContext(ctx, "files of the directory that are not in the archive", "files", strings.Join(left, ", "))
	}
	return nil
}

// requireValid runs the checks of check chart on files. When they find
// anything, it writes the report to stderr, which keeps stdout for the path of
// the archive, and returns an error that starts with failure.
func requireValid(cmd *cobra.Command, cfg *rules.Config, files *chart.Files, failure string) error {
	result, err := check.Chart(cfg, files, files.Metadata.Version)
	if err != nil {
		return err
	}
	if result.Valid {
		return nil
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), resultText(result)); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return errors.New(failure)
}
