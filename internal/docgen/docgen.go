// Package docgen renders the README commands table from the cobra command
// tree, whose help text comes from the embedded command docs.
package docgen

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	startMarker = "<!-- COMMANDS:START -->"
	endMarker   = "<!-- COMMANDS:END -->"

	// docsDir is where each command's markdown lives, relative to the README.
	docsDir = "cmd/docs"
)

// ErrMarkers is returned when the README lacks a well-formed marker pair.
var ErrMarkers = errors.New("README commands markers")

// Table renders a markdown table of the subcommands of root, each linking to
// its doc. Hidden and cobra's own help and completion commands are skipped.
func Table(root *cobra.Command) string {
	cmds := make([]*cobra.Command, 0, len(root.Commands()))
	for _, c := range root.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		cmds = append(cmds, c)
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name() < cmds[j].Name() })

	var b strings.Builder
	b.WriteString("| Command | Description |\n")
	b.WriteString("|---------|-------------|\n")
	for _, c := range cmds {
		fmt.Fprintf(&b, "| [`%s`](%s/%s.md) | %s |\n",
			c.Name(), docsDir, c.Name(), strings.ReplaceAll(c.Short, "|", `\|`))
	}
	return b.String()
}

// Replace returns readme with the text between the commands markers replaced
// by table. It fails unless the README holds exactly one start marker followed
// by exactly one end marker.
func Replace(readme, table string) (string, error) {
	if strings.Count(readme, startMarker) != 1 || strings.Count(readme, endMarker) != 1 {
		return "", fmt.Errorf("%w: want exactly one %s and one %s", ErrMarkers, startMarker, endMarker)
	}

	start := strings.Index(readme, startMarker)
	end := strings.Index(readme, endMarker)
	if end < start {
		return "", fmt.Errorf("%w: %s comes before %s", ErrMarkers, endMarker, startMarker)
	}

	return readme[:start+len(startMarker)] + "\n" + table + readme[end:], nil
}

// Update renders the commands table of root into readme.
func Update(readme string, root *cobra.Command) (string, error) {
	return Replace(readme, Table(root))
}
