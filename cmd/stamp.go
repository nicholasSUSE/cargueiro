package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/stamp"
)

func newStampCmd(opts *rootOptions, list listFunc) (*cobra.Command, error) {
	var newVersion, registry string
	var plainHTTP, dryRun bool
	var creds credentialFlags

	cmd := &cobra.Command{
		Use:  "stamp <chart-dir>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("version") && newVersion == "" {
				return errors.New("--version is empty: give a version, or leave the flag out to work it out")
			}
			given, err := creds.read(cmd, opts)
			if err != nil {
				return err
			}
			return stampChart(cmd, opts, args[0], dryRun, stamp.Options{
				Version:     newVersion,
				Registry:    registry,
				PlainHTTP:   plainHTTP,
				Credentials: given.credentialFunc(cmd.Context()),
				List:        stamp.ListFunc(list),
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&newVersion, "version", "", "the final version of the chart, 1XY.Z.W+upA.B.C (default: worked out from the versions released)")
	flags.StringVar(&registry, "registry", "", "registry path to read the released versions from, host[:port]/path, instead of the staging registry of the chart")
	flags.BoolVar(&plainHTTP, "plain-http", false, "talk HTTP to the registry instead of HTTPS, for local registries")
	creds.addFlags(flags)
	flags.BoolVar(&dryRun, "dry-run", false, "print what would change as a diff, and write nothing")
	if err := applyDoc(cmd, "stamp"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// printDryRun prints what stamping would change, as a diff, and writes nothing.
func printDryRun(cmd *cobra.Command, plan *stamp.Plan) error {
	out := plan.Diff()
	if out == "" {
		out = "nothing to change\n"
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), out); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

// stampChart puts the version and the helm.sh/images annotation on the chart in
// dir, and prints what changed. Nothing is written unless every check passes.
func stampChart(cmd *cobra.Command, opts *rootOptions, dir string, dryRun bool, stampOpts stamp.Options) error {
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "stamping chart", "chart", dir, "version", stampOpts.Version)

	plan, err := stamp.Build(cmd.Context(), cfg, dir, stampOpts)
	if err != nil {
		return err
	}
	if dryRun {
		return printDryRun(cmd, plan)
	}
	if err := plan.Write(); err != nil {
		return err
	}

	var report strings.Builder
	if plan.OldVersion != plan.NewVersion {
		fmt.Fprintf(&report, "version: %s -> %s\n", plan.OldVersion, plan.NewVersion)
	}
	if len(plan.AddedImages) > 0 {
		fmt.Fprintf(&report, "helm.sh/images: added %s\n", strings.Join(plan.AddedImages, ", "))
	}
	if report.Len() == 0 {
		report.WriteString("nothing to change\n")
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), report.String()); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}
