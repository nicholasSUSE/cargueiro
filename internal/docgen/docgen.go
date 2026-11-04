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

// Table renders a markdown table of every command below root, nested ones
// included, each linking to its doc. A nested command such as "registry login"
// links to registry-login.md. Hidden commands and cobra's own help and
// completion are skipped.
func Table(root *cobra.Command) string {
	type row struct{ path, short string }
	var rows []row

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, child := range c.Commands() {
			if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			path := strings.TrimPrefix(child.CommandPath(), root.CommandPath()+" ")
			rows = append(rows, row{path: path, short: child.Short})
			walk(child)
		}
	}
	walk(root)
	sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })

	var b strings.Builder
	b.WriteString("| Command | Description |\n")
	b.WriteString("|---------|-------------|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| [`%s`](%s/%s.md) | %s |\n",
			r.path, docsDir, strings.ReplaceAll(r.path, " ", "-"), strings.ReplaceAll(r.short, "|", `\|`))
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
