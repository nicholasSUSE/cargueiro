package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/cmd/docs"
)

// isBuiltin reports whether name is a command added by cobra itself, which
// has no embedded doc.
func isBuiltin(name string) bool {
	return name == "help" || name == "completion"
}

func TestEveryCommandHasADocAndViceVersa(t *testing.T) {
	root, err := NewRootCmd(&bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)

	var commands []string
	for _, c := range root.Commands() {
		if !isBuiltin(c.Name()) && !c.Hidden {
			commands = append(commands, c.Name())
		}
	}

	documented, err := docs.Names()
	require.NoError(t, err)

	require.ElementsMatch(t, commands, documented)
}

func TestEveryLocalFlagIsDocumented(t *testing.T) {
	root, err := NewRootCmd(&bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)

	for _, c := range root.Commands() {
		if isBuiltin(c.Name()) {
			continue
		}

		t.Run(c.Name(), func(t *testing.T) {
			doc, err := docs.Load(c.Name())
			require.NoError(t, err)

			c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
				if f.Name == "help" {
					return
				}
				require.Truef(t, strings.Contains(doc.Source, "--"+f.Name),
					"flag --%s of %q is not mentioned in cmd/docs/%s.md", f.Name, c.Name(), c.Name())
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
