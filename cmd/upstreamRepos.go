package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/rules"
)

// noUpstream is what is printed for a main chart whose upstream is not set.
const noUpstream = "needs an upstream"

func newUpstreamReposCmd(opts *rootOptions) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:  "upstream-repos",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := rules.Parse(opts.rules)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(cmd.OutOrStdout(), upstreamReposText(cfg.MainCharts())); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
			return nil
		},
	}
	if err := applyDoc(cmd, "upstream-repos"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// upstreamReposText prints a line for each main chart: its name and its
// upstream, or that it needs one, with the names padded to the same width.
func upstreamReposText(mains []rules.Chart) string {
	width := 0
	for _, chart := range mains {
		width = max(width, len(chart.Name))
	}

	var b strings.Builder
	for _, chart := range mains {
		upstream := chart.Upstream
		if upstream == "" {
			upstream = noUpstream
		}
		fmt.Fprintf(&b, "%-*s  %s\n", width, chart.Name, upstream)
	}
	return b.String()
}
