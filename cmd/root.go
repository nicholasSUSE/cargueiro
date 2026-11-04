// Package cmd wires the cargueiro command line interface.
package cmd

import (
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/logger"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// rootOptions holds state shared by all subcommands.
type rootOptions struct {
	debug  bool
	stderr io.Writer
	log    *slog.Logger
	getenv func(string) string
	// openStore opens the saved registry credentials.
	openStore func() (*auth.Store, error)
	// prompt asks the user for input.
	prompt prompter
}

// dependencies are the collaborators commands call, replaced in tests.
type dependencies struct {
	pkg         packageFunc
	newPusher   pusherFactory
	verifyLogin verifyFunc
}

// NewRootCmd builds the root command. Output streams are injected so
// commands stay testable.
func NewRootCmd(stdout, stderr io.Writer) (*cobra.Command, error) {
	opts := &rootOptions{
		stderr:    stderr,
		getenv:    os.Getenv,
		openStore: defaultStore,
		prompt:    newTerminalPrompter(os.Stdin, stderr),
	}
	return newRootCmd(stdout, stderr, opts, dependencies{
		pkg:         chart.Package,
		newPusher:   newOCIPusher,
		verifyLogin: oci.VerifyLogin,
	})
}

// newRootCmd is NewRootCmd with its dependencies injected.
func newRootCmd(stdout, stderr io.Writer, opts *rootOptions, deps dependencies) (*cobra.Command, error) {
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

	versionCmd, err := newVersionCmd()
	if err != nil {
		return nil, err
	}
	packageCmd, err := newPackageCmd(opts, deps.pkg)
	if err != nil {
		return nil, err
	}
	pushCmd, err := newPushCmd(opts, deps.newPusher)
	if err != nil {
		return nil, err
	}
	registryCmd, err := newRegistryCmd(opts, deps.verifyLogin)
	if err != nil {
		return nil, err
	}
	root.AddCommand(versionCmd, packageCmd, pushCmd, registryCmd)

	return root, nil
}
