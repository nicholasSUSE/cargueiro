package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/cmd/docs"
)

// isBuiltin reports whether name is a command added by cobra itself, which
// has no embedded doc.
func isBuiltin(name string) bool {
	return name == "help" || name == "completion"
}

// documented returns every command below root that must have a doc, with the
// doc name it must have: "registry login" is registry-login.
func documented(root *cobra.Command) map[string]*cobra.Command {
	found := map[string]*cobra.Command{}

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, child := range c.Commands() {
			if isBuiltin(child.Name()) || child.Hidden {
				continue
			}
			path := strings.TrimPrefix(child.CommandPath(), root.CommandPath()+" ")
			found[strings.ReplaceAll(path, " ", "-")] = child
			walk(child)
		}
	}
	walk(root)
	return found
}

func TestEveryCommandHasADocAndViceVersa(t *testing.T) {
	root, err := NewRootCmd(&bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)

	var commands []string
	for name := range documented(root) {
		commands = append(commands, name)
	}

	docNames, err := docs.Names()
	require.NoError(t, err)

	require.ElementsMatch(t, commands, docNames)
}

func TestEveryLocalFlagIsDocumented(t *testing.T) {
	root, err := NewRootCmd(&bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)

	for name, c := range documented(root) {
		t.Run(name, func(t *testing.T) {
			doc, err := docs.Load(name)
			require.NoError(t, err)

			c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
				if f.Name == "help" {
					return
				}
				require.Truef(t, strings.Contains(doc.Source, "--"+f.Name),
					"flag --%s of %q is not mentioned in cmd/docs/%s.md", f.Name, c.CommandPath(), name)
			})
		})
	}
}

func TestHelpComesFromEmbeddedDocs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root, err := NewRootCmd(&stdout, &stderr)
	require.NoError(t, err)
	root.SetArgs([]string{"package", "--help"})

	require.NoError(t, root.Execute())

	doc, err := docs.Load("package")
	require.NoError(t, err)
	require.Contains(t, stdout.String(), doc.Short)
	require.Contains(t, stdout.String(), "cargueiro package charts/fleet/110.0.3+up0.16.3")
	require.Contains(t, stdout.String(), "--output-dir")
}
