package cmd

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// Only `registry login` reads a password from standard input: every other
// command takes it from the environment or from the saved login.
func TestOnlyRegistryLoginHasPasswordStdin(t *testing.T) {
	root, err := NewRootCmd(io.Discard, io.Discard, nil)
	require.NoError(t, err)

	var with []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Flags().Lookup("password-stdin") != nil {
			with = append(with, c.CommandPath())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)

	require.Len(t, with, 1, with)
	require.True(t, strings.HasSuffix(with[0], "registry login"), with[0])
}
