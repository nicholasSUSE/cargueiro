package chart

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// scalarLine returns the expression that matches the top-level key of a
// Chart.yaml on one line: the key, its value (plain, or in single or double
// quotes) and an optional comment. Indented keys, such as one under annotations,
// do not match, and neither do keys that only start with the same letters, such
// as appVersion for version.
func scalarLine(key string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `:([ \t]+)("[^"]*"|'[^']*'|[^\s#"'|>][^\s#]*)([ \t]*#.*)?$`)
}

// SetVersion returns chartYAML with its version changed to version. Only that
// line changes: every other byte, comments and line endings included, stays as
// it is, so a diff shows nothing else. A trailing comment on the line is kept.
//
// It fails when Chart.yaml has no top-level version on one line, or has more
// than one, because then it cannot say which to change. It also fails if the
// result would not read back as the text of the version asked for: a version
// such as 123 would be a number.
func SetVersion(chartYAML []byte, version string) ([]byte, error) {
	return setScalar(chartYAML, "version", version)
}

// setScalar returns chartYAML with the value of the top-level key changed to
// value, as SetVersion does for the version.
func setScalar(chartYAML []byte, key, value string) ([]byte, error) {
	lines := bytes.SplitAfter(chartYAML, []byte("\n"))

	found := -1
	for i, line := range lines {
		if !bytes.HasPrefix(line, []byte(key+":")) {
			continue
		}
		if found >= 0 {
			return nil, fmt.Errorf("the Chart.yaml has more than one %s", key)
		}
		found = i
	}
	if found < 0 {
		return nil, fmt.Errorf("the Chart.yaml has no %s", key)
	}

	line, ending := splitEnding(lines[found])
	match := scalarLine(key).FindSubmatch(line)
	if match == nil {
		return nil, fmt.Errorf("the %s line of the Chart.yaml is not a value on one line: %q", key, line)
	}

	var edited bytes.Buffer
	for i, l := range lines {
		if i != found {
			edited.Write(l)
			continue
		}
		edited.WriteString(key + ":")
		edited.Write(match[1])
		edited.WriteString(value)
		edited.Write(match[3])
		edited.Write(ending)
	}

	var back map[string]yaml.Node
	if err := yaml.Unmarshal(edited.Bytes(), &back); err != nil {
		return nil, fmt.Errorf("the Chart.yaml would not be valid YAML with %s %q: %w", key, value, err)
	}
	if node := back[key]; node.Tag != "!!str" || node.Value != value {
		return nil, fmt.Errorf("the Chart.yaml would not read %q as the text of the %s", value, key)
	}
	return edited.Bytes(), nil
}

// splitEnding separates the line from its line ending: "\n", "\r\n" or none.
func splitEnding(line []byte) (text, ending []byte) {
	switch {
	case bytes.HasSuffix(line, []byte("\r\n")):
		return line[:len(line)-2], line[len(line)-2:]
	case bytes.HasSuffix(line, []byte("\n")):
		return line[:len(line)-1], line[len(line)-1:]
	}
	return line, nil
}

// annotationsLine matches the top-level annotations key of a Chart.yaml when
// its mapping is written as a block, and annotationsEmpty when it is written as
// an empty flow mapping.
var (
	annotationsLine  = regexp.MustCompile(`^annotations:[ \t]*(#.*)?$`)
	annotationsEmpty = regexp.MustCompile(`^annotations:[ \t]*\{\}[ \t]*(#.*)?$`)
)

// SetAnnotation returns chartYAML with the annotation key set to value, written
// as a literal block (key: |). Only the lines of that annotation change: if
// the key exists its lines are replaced, otherwise it is added at the end of
// the annotations, and if there are no annotations a section is added at the end
// of the file. Every other line, comments included, stays as it is.
//
// It fails when it cannot tell where the annotations are: a mapping written in
// flow style with content, or the key twice. It also fails if the result would
// not read back with the value asked for.
func SetAnnotation(chartYAML []byte, key, value string) ([]byte, error) {
	if value == "" {
		return nil, errors.New("an annotation needs a value")
	}
	if value[0] == ' ' || value[0] == '\t' || value[0] == '\n' {
		return nil, errors.New("the value of an annotation cannot start with a space or a new line")
	}

	lines := bytes.SplitAfter(chartYAML, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	ending := "\n"
	for _, l := range lines {
		if bytes.HasSuffix(l, []byte("\r\n")) {
			ending = "\r\n"
			break
		}
	}

	section := -1
	for i, l := range lines {
		text, _ := splitEnding(l)
		if !bytes.HasPrefix(text, []byte("annotations:")) {
			continue
		}
		if section >= 0 {
			return nil, errors.New("the Chart.yaml has more than one annotations")
		}
		if !annotationsLine.Match(text) && !annotationsEmpty.Match(text) {
			return nil, fmt.Errorf("the annotations of the Chart.yaml are not a block mapping: %q", text)
		}
		section = i
	}

	var out [][]byte
	switch {
	case section < 0:
		out = append(out, lines...)
		if n := len(out); n > 0 && !bytes.HasSuffix(out[n-1], []byte("\n")) {
			out[n-1] = append(append([]byte{}, out[n-1]...), ending...)
		}
		out = append(out, []byte("annotations:"+ending))
		out = append(out, renderAnnotation(key, value, 2, ending)...)
	default:
		var err error
		if out, err = placeAnnotation(lines, section, key, value, ending); err != nil {
			return nil, err
		}
	}

	edited := bytes.Join(out, nil)

	var back struct {
		Annotations map[string]string `yaml:"annotations"`
	}
	if err := yaml.Unmarshal(edited, &back); err != nil {
		return nil, fmt.Errorf("the Chart.yaml would not be valid YAML with the annotation %q: %w", key, err)
	}
	if back.Annotations[key] != value+"\n" && back.Annotations[key] != value {
		return nil, fmt.Errorf("the Chart.yaml would not read the annotation %q with the value asked for", key)
	}
	return edited, nil
}

// placeAnnotation sets the annotation in the block that starts at line section.
func placeAnnotation(lines [][]byte, section int, key, value, ending string) ([][]byte, error) {
	header, _ := splitEnding(lines[section])
	empty := annotationsEmpty.Match(header)

	// The block runs to the next top-level key.
	end := len(lines)
	if !empty {
		for i := section + 1; i < len(lines); i++ {
			text, _ := splitEnding(lines[i])
			if len(text) > 0 && text[0] != ' ' && text[0] != '\t' && text[0] != '#' {
				end = i
				break
			}
		}
	}

	indent, last := 0, section
	for i := section + 1; i < end; i++ {
		text, _ := splitEnding(lines[i])
		if len(bytes.TrimSpace(text)) == 0 {
			continue
		}
		last = i
		if text[0] != '#' && indent == 0 {
			indent = leadingSpaces(text)
		}
	}
	if indent == 0 {
		indent = 2
	}

	found := -1
	for i := section + 1; i < end; i++ {
		text, _ := splitEnding(lines[i])
		if leadingSpaces(text) != indent || !bytes.HasPrefix(text[indent:], []byte(key+":")) {
			continue
		}
		if found >= 0 {
			return nil, fmt.Errorf("the annotation %q is in the Chart.yaml twice", key)
		}
		found = i
	}

	block := renderAnnotation(key, value, indent, ending)
	var out [][]byte
	if empty {
		out = append(out, []byte("annotations:"+ending))
		out = append(out, block...)
		return append(out, lines[section+1:]...), nil
	}

	switch {
	case found < 0:
		out = append(out, lines[:last+1]...)
		if !bytes.HasSuffix(lines[last], []byte("\n")) {
			out[len(out)-1] = append(append([]byte{}, lines[last]...), ending...)
		}
		out = append(out, block...)
		return append(out, lines[last+1:]...), nil
	default:
		stop := found + 1
		for stop < end {
			text, _ := splitEnding(lines[stop])
			if len(bytes.TrimSpace(text)) != 0 && leadingSpaces(text) <= indent {
				break
			}
			stop++
		}
		for stop > found+1 && len(bytes.TrimSpace(lines[stop-1])) == 0 {
			stop--
		}
		out = append(out, lines[:found]...)
		out = append(out, block...)
		return append(out, lines[stop:]...), nil
	}
}

// renderAnnotation writes the annotation as a literal block, each line of the
// value indented two spaces more than the key.
func renderAnnotation(key, value string, indent int, ending string) [][]byte {
	pad := bytes.Repeat([]byte(" "), indent)
	lines := [][]byte{[]byte(string(pad) + key + ": |" + ending)}
	for _, line := range bytes.Split([]byte(trimNewlines(value)), []byte("\n")) {
		if len(line) == 0 {
			lines = append(lines, []byte(ending))
			continue
		}
		lines = append(lines, []byte(string(pad)+"  "+string(line)+ending))
	}
	return lines
}

// trimNewlines removes the new lines at the end of s.
func trimNewlines(s string) string {
	for len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s
}

// leadingSpaces counts the spaces at the start of text.
func leadingSpaces(text []byte) int {
	n := 0
	for n < len(text) && text[n] == ' ' {
		n++
	}
	return n
}
