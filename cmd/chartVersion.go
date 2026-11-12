package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/chartversion"
	"github.com/rancher/cargueiro/internal/rules"
)

func newChartVersionCmd(opts *rootOptions, list listFunc) (*cobra.Command, error) {
	var registry string
	var plainHTTP bool
	var creds credentialFlags

	cmd := &cobra.Command{
		Use:  "chart-version <chart-dir>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			given := creds.read(opts)
			return printChartVersion(cmd, opts, args[0], chartversion.Options{
				Registry:    registry,
				PlainHTTP:   plainHTTP,
				Credentials: given.credentialFunc(cmd.Context()),
				List:        chartversion.ListFunc(list),
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&registry, "registry", "", "registry path to read the released versions from, host[:port]/path, instead of the staging registry of the chart")
	flags.BoolVar(&plainHTTP, "plain-http", false, "talk HTTP to the registry instead of HTTPS, for local registries")
	creds.addFlags(flags)
	if err := applyDoc(cmd, "chart-version"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// printChartVersion prints the final version of the next release of the chart in
// dir, and writes nothing.
func printChartVersion(cmd *cobra.Command, opts *rootOptions, dir string, versionOpts chartversion.Options) error {
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "working out the version", "chart", dir)

	next, err := chartversion.Version(cmd.Context(), cfg, dir, versionOpts)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), next); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}
