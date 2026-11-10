// Package cmd wires the cargueiro command line interface.
package cmd

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/auth"
	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/checkout"
	"github.com/rancher/cargueiro/internal/logger"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/ship"
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
	// rules is the content of rules.yaml, which says what each chart must be.
	rules []byte
}

// dependencies are the collaborators commands call, replaced in tests.
type dependencies struct {
	pkg         packageFunc
	newPusher   pusherFactory
	verifyLogin verifyFunc
	fetchChart  fetchFunc
	listTags    listFunc
	prepare     prepareFunc
	// verifyCheckout checks the charts repository that ship writes to.
	verifyCheckout verifyCheckoutFunc
}

// NewRootCmd builds the root command. Output streams are injected so
// commands stay testable. rules is the content of rules.yaml.
func NewRootCmd(stdout, stderr io.Writer, rules []byte) (*cobra.Command, error) {
	opts := &rootOptions{
		stderr:    stderr,
		getenv:    os.Getenv,
		openStore: defaultStore,
		prompt:    newTerminalPrompter(os.Stdin, stderr),
		rules:     rules,
	}
	return newRootCmd(stdout, stderr, opts, dependencies{
		pkg:         chart.Package,
		newPusher:   newOCIPusher,
		verifyLogin: oci.VerifyLogin,
		fetchChart:  oci.FetchChart,
		listTags:    oci.ListTags,
		prepare:     ship.Prepare,
		verifyCheckout: func(ctx context.Context, dir, branch string) error {
			return checkout.Verify(ctx, dir, branch, checkout.Git)
		},
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
	checkCmd, err := newCheckCmd(opts, deps.fetchChart)
	if err != nil {
		return nil, err
	}
	listCmd, err := newListCmd(opts, deps.listTags)
	if err != nil {
		return nil, err
	}
	stampCmd, err := newStampCmd(opts, deps.listTags)
	if err != nil {
		return nil, err
	}
	indexCmd, err := newIndexCmd(opts)
	if err != nil {
		return nil, err
	}
	shipCmd, err := newShipCmd(opts, deps.prepare, deps.verifyCheckout)
	if err != nil {
		return nil, err
	}
	root.AddCommand(versionCmd, packageCmd, pushCmd, registryCmd, checkCmd, listCmd, stampCmd, indexCmd, shipCmd)

	return root, nil
}
