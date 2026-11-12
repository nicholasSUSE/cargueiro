package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/chartversion"
	"github.com/rancher/cargueiro/internal/family"
	"github.com/rancher/cargueiro/internal/rules"
)

func newChartVersionCmd(opts *rootOptions, list listFunc) (*cobra.Command, error) {
	var registry, familyName string
	var plainHTTP bool
	var creds credentialFlags

	cmd := &cobra.Command{
		Use: "chart-version <chart-dir>",
		Args: func(cmd *cobra.Command, args []string) error {
			return checkArgs(familyName)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			given := creds.read(opts)
			versionOpts := chartversion.Options{
				Registry:    registry,
				PlainHTTP:   plainHTTP,
				Credentials: given.credentialFunc(cmd.Context()),
				List:        chartversion.ListFunc(list),
			}
			if familyName != "" {
				return printFamilyVersion(cmd, opts, familyName, args, versionOpts)
			}
			return printChartVersion(cmd, opts, args[0], versionOpts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&familyName, "family", "", "work out the version of the family of this name, with one folder for each of its charts")
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

// printFamilyVersion prints the version of the family called name: dirs must be
// the folders of exactly the charts of the family, in any order. The version is
// the one of its main chart, and every other chart takes it.
func printFamilyVersion(cmd *cobra.Command, opts *rootOptions, name string, dirs []string, versionOpts chartversion.Options) error {
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	members, err := cfg.Family(name)
	if err != nil {
		return err
	}
	loaded, err := loadAll(dirs, chart.Load)
	if err != nil {
		return err
	}
	ordered, err := family.Resolve(members, loaded, chartName)
	if err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "working out the version of the family", "family", name)

	next, err := chartversion.FamilyVersion(cmd.Context(), cfg, members, ordered[0].path, versionOpts)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), next); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}
