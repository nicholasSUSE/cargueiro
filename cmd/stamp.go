package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/rules"
	"github.com/nicholasSUSE/cargueiro/internal/stamp"
)

func newStampCmd(opts *rootOptions) (*cobra.Command, error) {
	var newVersion string

	cmd := &cobra.Command{
		Use:  "stamp <chart-dir>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return stampChart(cmd, opts, args[0], newVersion)
		},
	}
	cmd.Flags().StringVar(&newVersion, "version", "", "the final version of the chart, 1XY.Z.W+upA.B.C")
	if err := cmd.MarkFlagRequired("version"); err != nil {
		return nil, fmt.Errorf("mark --version required: %w", err)
	}
	if err := applyDoc(cmd, "stamp"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// stampChart puts the version and the helm.sh/images annotation on the chart in
// dir, and prints what changed. Nothing is written unless every check passes.
func stampChart(cmd *cobra.Command, opts *rootOptions, dir, newVersion string) error {
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "stamping chart", "chart", dir, "version", newVersion)

	plan, err := stamp.Build(cmd.Context(), cfg, dir, stamp.Options{Version: newVersion})
	if err != nil {
		return err
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
