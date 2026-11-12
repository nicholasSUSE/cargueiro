package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/family"
	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/stamp"
)

// stampFamily stamps the family called name: dirs must be the folders of exactly
// the charts of the family, in any order. The version of the main chart is the
// version of the family. Nothing is written unless every check passes.
func stampFamily(cmd *cobra.Command, opts *rootOptions, name string, dirs []string, dryRun bool, stampOpts stamp.Options) error {
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
	paths := make([]string, 0, len(ordered))
	for _, l := range ordered {
		paths = append(paths, l.path)
	}
	opts.log.DebugContext(cmd.Context(), "stamping family", "family", name, "version", stampOpts.Version)

	plan, err := stamp.BuildFamily(cfg, members, paths, stampOpts)
	if err != nil {
		return err
	}
	if !dryRun {
		if err := plan.Write(); err != nil {
			return err
		}
	}
	for _, c := range plan.Charts {
		text := reportText(c.Plan)
		if dryRun {
			text = dryRunText(c.Plan)
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "==> %s\n%s", c.Chart, text); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}
	return nil
}
