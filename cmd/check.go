package cmd

import (
	"github.com/spf13/cobra"
)

// newCheckCmd groups the commands that check a chart against the rules.
func newCheckCmd(opts *rootOptions) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:  "check",
		Args: cobra.NoArgs,
	}
	if err := applyDoc(cmd, "check"); err != nil {
		return nil, err
	}

	chart, err := newCheckChartCmd(opts)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(chart)

	return cmd, nil
}
