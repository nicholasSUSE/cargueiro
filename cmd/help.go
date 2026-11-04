package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/cmd/docs"
)

// applyDoc sets the help text of cmd from the embedded doc named name.
func applyDoc(cmd *cobra.Command, name string) error {
	doc, err := docs.Load(name)
	if err != nil {
		return fmt.Errorf("load docs for %s: %w", name, err)
	}

	cmd.Short = doc.Short
	cmd.Long = doc.Long
	cmd.Example = indent(doc.Example)
	return nil
}

// indent prefixes every non-empty line with two spaces, the way cobra
// examples are conventionally shown.
func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}
