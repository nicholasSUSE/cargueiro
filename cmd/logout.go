package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogoutCmd(opts *rootOptions) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:  "logout",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := opts.openStore()
			if err != nil {
				return err
			}

			// The file may be corrupt; logout still removes it.
			login, known, _ := store.Load()

			removed, err := store.Delete()
			if err != nil {
				return err
			}

			switch {
			case !removed:
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "No saved login")
			case known:
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed the saved login for %s\n", login.Host)
			default:
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "Removed the saved login")
			}
			return err
		},
	}

	if err := applyDoc(cmd, "registry-logout"); err != nil {
		return nil, err
	}
	return cmd, nil
}
