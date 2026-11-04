// Package docs holds the embedded command documentation, the single source of
// truth for the text shown by --help and linked from the README.
//
// Each command has one markdown file named after it, with this layout. A nested
// command such as "registry login" is stored as registry-login.md with the
// title "registry login".
//
//	# <command>
//
//	<first paragraph: the short description>
//
//	<long description, up to "## Examples">
//
//	## Examples
//
//	```
//	<example commands, fences are stripped for --help>
//	```
package docs

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed *.md
var files embed.FS

var (
	// ErrNotFound is returned when a command has no embedded doc.
	ErrNotFound = errors.New("command doc not found")
	// ErrMalformed is returned when a doc does not follow the expected layout.
	ErrMalformed = errors.New("malformed command doc")
)

const examplesHeading = "## Examples"

// Doc is the parsed documentation of one command.
type Doc struct {
	Short   string // one-line description
	Long    string // full description
	Example string // example commands, without markdown fences
	Source  string // raw markdown, used to check that flags are documented
}

// Load returns the documentation embedded for the named command.
func Load(name string) (Doc, error) {
	src, err := files.ReadFile(name + ".md")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Doc{}, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return Doc{}, fmt.Errorf("read doc %s: %w", name, err)
	}

	return parse(name, string(src))
}

// Names returns the command names that have an embedded doc.
func Names() ([]string, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("list docs: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".md"))
	}
	return names, nil
}

func parse(name, src string) (Doc, error) {
	// A nested command "registry login" lives in registry-login.md.
	title := "# " + strings.ReplaceAll(name, "-", " ")
	body, ok := strings.CutPrefix(strings.TrimSpace(src), title+"\n")
	if !ok {
		return Doc{}, fmt.Errorf("%w: %s must start with %q", ErrMalformed, name, title)
	}

	main, examples, hasExamples := strings.Cut(body, "\n"+examplesHeading+"\n")
	paragraphs := strings.SplitN(strings.TrimSpace(main), "\n\n", 2)

	short := strings.Join(strings.Fields(paragraphs[0]), " ")
	if short == "" {
		return Doc{}, fmt.Errorf("%w: %s has no short description", ErrMalformed, name)
	}

	doc := Doc{Short: short, Long: short, Source: src}
	if len(paragraphs) == 2 {
		doc.Long = short + "\n\n" + strings.TrimSpace(paragraphs[1])
	}

	if hasExamples {
		example, err := fencedBlocks(examples)
		if err != nil {
			return Doc{}, fmt.Errorf("%w: %s: %w", ErrMalformed, name, err)
		}
		doc.Example = example
	}

	return doc, nil
}

// fencedBlocks returns the contents of every ``` block in s, separated by a
// blank line. Text outside the fences is ignored.
func fencedBlocks(s string) (string, error) {
	var (
		blocks  []string
		current []string
		inside  bool
	)
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "```") {
			if inside {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
			}
			inside = !inside
			continue
		}
		if inside {
			current = append(current, line)
		}
	}

	if inside {
		return "", errors.New("unclosed code fence in examples")
	}
	if len(blocks) == 0 {
		return "", errors.New("examples section has no code block")
	}
	return strings.Join(blocks, "\n\n"), nil
}
