package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/ship"
)

// prepareFunc works out what to ship for the chart at ref to the charts
// repository in dir.
type prepareFunc func(ctx context.Context, cfg *rules.Config, ref oci.ChartRef, dir string, opts ship.Options) (ship.Shipment, error)

// verifyCheckoutFunc checks that dir is the checkout of branch that it must be.
type verifyCheckoutFunc func(ctx context.Context, dir, branch string) error

// defaultBranch is the branch of the charts repository that entries are shipped to.
const defaultBranch = "staging"

func newShipCmd(opts *rootOptions, prepare prepareFunc, verifyCheckout verifyCheckoutFunc) (*cobra.Command, error) {
	var repoDir, branch string
	var plainHTTP, dryRun bool
	var creds credentialFlags

	cmd := &cobra.Command{
		Use:  "ship <registry/path/chart:tag>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return shipChart(cmd, opts, prepare, verifyCheckout, args[0], repoDir, branch, plainHTTP, dryRun, &creds)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&repoDir, "repo", "", "absolute path of the charts repository to ship to")
	flags.StringVar(&branch, "branch", defaultBranch, "the branch of the charts repository that --repo must be on, and be up to date with")
	flags.BoolVar(&plainHTTP, "plain-http", false, "talk HTTP to the registry instead of HTTPS, for local registries")
	flags.BoolVar(&dryRun, "dry-run", false, "print the files that would be written, and write nothing")
	creds.addFlags(flags)
	if err := cmd.MarkFlagRequired("repo"); err != nil {
		return nil, fmt.Errorf("mark --repo required: %w", err)
	}
	if err := applyDoc(cmd, "ship"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// shipChart writes the entry, and the logo if there is one, of the chart at
// reference to the charts repository in repoDir, and prints the files.
func shipChart(cmd *cobra.Command, opts *rootOptions, prepare prepareFunc, verifyCheckout verifyCheckoutFunc, reference, repoDir, branch string, plainHTTP, dryRun bool, creds *credentialFlags) error {
	if !filepath.IsAbs(repoDir) {
		return fmt.Errorf("--repo must be an absolute path, got %q", repoDir)
	}
	ref, err := oci.ParseChartRef(reference)
	if err != nil {
		return err
	}
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "shipping chart", "reference", ref.String(), "repo", repoDir, "branch", branch, "dryRun", dryRun)

	if err := verifyCheckout(cmd.Context(), repoDir, branch); err != nil {
		return err
	}

	given, err := creds.read(cmd, opts)
	if err != nil {
		return err
	}

	shipment, err := prepare(cmd.Context(), cfg, ref, repoDir, ship.Options{PlainHTTP: plainHTTP, Credentials: given.credentialFunc(cmd.Context())})
	var invalid *ship.InvalidChartError
	if errors.As(err, &invalid) {
		if _, werr := fmt.Fprint(cmd.ErrOrStderr(), resultText(invalid.Result)); werr != nil {
			return fmt.Errorf("write result: %w", werr)
		}
	}
	if err != nil {
		return err
	}

	if !dryRun {
		if err := shipment.Write(repoDir); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), strings.Join(shipment.Paths(), "\n")); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}
