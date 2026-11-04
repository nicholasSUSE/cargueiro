package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogoutCmd(opts *rootOptions) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:  "logout <host>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host, err := registryHost(args[0])
			if err != nil {
				return err
			}

			store, err := opts.openStore()
			if err != nil {
				return fmt.Errorf("open the credentials file: %w", err)
			}

			removed, err := store.Delete(cmd.Context(), host)
			if err != nil {
				return err
			}

			if removed {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed credentials for %s\n", host)
			} else {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "No credentials stored for %s\n", host)
			}
			return err
		},
	}

	if err := applyDoc(cmd, "registry-logout"); err != nil {
		return nil, err
	}
	return cmd, nil
}
