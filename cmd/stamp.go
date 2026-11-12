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
	var newVersion, registry, familyName string
	var plainHTTP, dryRun bool
	var creds credentialFlags

	cmd := &cobra.Command{
		Use: "stamp <chart-dir>",
		Args: func(cmd *cobra.Command, args []string) error {
			return checkArgs(familyName)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("version") && newVersion == "" {
				return errors.New("--version is empty: give a version, or leave the flag out to work it out")
			}
			given := creds.read(opts)
			stampOpts := stamp.Options{
				Version:     newVersion,
				Registry:    registry,
				PlainHTTP:   plainHTTP,
				Credentials: given.credentialFunc(cmd.Context()),
				List:        stamp.ListFunc(list),
			}
			if familyName != "" {
				return stampFamily(cmd, opts, familyName, args, dryRun, stampOpts)
			}
			return stampChart(cmd, opts, args[0], dryRun, stampOpts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&familyName, "family", "", "stamp the family of this name, with one folder for each of its charts")
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
	if _, err := fmt.Fprint(cmd.OutOrStdout(), dryRunText(plan)); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

// dryRunText is what stamping would change, as a diff.
func dryRunText(plan *stamp.Plan) string {
	if out := plan.Diff(); out != "" {
		return out
	}
	return "nothing to change\n"
}

// reportText is what stamping changed, one line for each change.
func reportText(plan *stamp.Plan) string {
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
	return report.String()
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

	if _, err := fmt.Fprint(cmd.OutOrStdout(), reportText(plan)); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}
