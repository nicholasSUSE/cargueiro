package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/chart"
)

func newCheckArchiveCmd(opts *rootOptions) (*cobra.Command, error) {
	var output string
	var noErrorHIP15 bool

	cmd := &cobra.Command{
		Use:  "archive <chart.tgz>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkArchive(cmd, opts, args[0], output, noErrorHIP15)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&output, "output", "o", outputText, "output format: text or json")
	flags.BoolVar(&noErrorHIP15, "no-error-hip-15", false, "do not fail because of helm.sh/images (HIP-15) errors")
	if err := applyDoc(cmd, "check-archive"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// checkArchive reads the chart archive at path and reports on it.
func checkArchive(cmd *cobra.Command, opts *rootOptions, path, output string, noErrorHIP15 bool) error {
	if err := checkOutputFormat(output); err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "checking chart", "archive", path, "output", output)

	files, err := loadArchive(path)
	if err != nil {
		return err
	}
	return report(cmd, opts, files, files.Metadata.Version, output, noErrorHIP15)
}

// loadArchive reads the chart archive at path.
func loadArchive(path string) (*chart.Files, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s not found", path)
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	files, err := chart.LoadArchive(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return files, nil
}
