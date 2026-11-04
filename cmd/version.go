package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/version"
)

func newVersionCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:  "version",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.Get())
			return err
		},
	}
	if err := applyDoc(cmd, "version"); err != nil {
		return nil, err
	}

	return cmd, nil
}
