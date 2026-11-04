package cmd

import (
	"github.com/spf13/cobra"
)

// newRegistryCmd groups the commands that manage saved registry credentials.
func newRegistryCmd(opts *rootOptions, verify verifyFunc) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:  "registry",
		Args: cobra.NoArgs,
	}
	if err := applyDoc(cmd, "registry"); err != nil {
		return nil, err
	}

	login, err := newLoginCmd(opts, verify)
	if err != nil {
		return nil, err
	}
	logout, err := newLogoutCmd(opts)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(login, logout)

	return cmd, nil
}
