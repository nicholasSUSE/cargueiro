// Package cmd wires the cargueiro command line interface.
package cmd

import (
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/logger"
)

// rootOptions holds state shared by all subcommands.
type rootOptions struct {
	debug  bool
	stderr io.Writer
	log    *slog.Logger
}

// NewRootCmd builds the root command. Output streams are injected so
// commands stay testable.
func NewRootCmd(stdout, stderr io.Writer) *cobra.Command {
	return newRootCmd(stdout, stderr, &rootOptions{stderr: stderr}, chart.Package)
}

// newRootCmd is NewRootCmd with its dependencies injected.
func newRootCmd(stdout, stderr io.Writer, opts *rootOptions, pkg packageFunc) *cobra.Command {
	root := &cobra.Command{
		Use:           "cargueiro",
		Short:         "Package Helm charts and push them to OCI registries",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(*cobra.Command, []string) {
			opts.log = logger.New(opts.stderr, opts.debug)
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().BoolVar(&opts.debug, "debug", false, "enable debug logging")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newPackageCmd(opts, pkg))

	return root
}
