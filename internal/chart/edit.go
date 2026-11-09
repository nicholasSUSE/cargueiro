package chart

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// versionLine matches the top-level version key of a Chart.yaml on one line:
// the key, its value (plain, or in single or double quotes) and an optional
// comment. Indented keys, such as one under annotations, do not match, and
// neither do appVersion or kubeVersion.
var versionLine = regexp.MustCompile(`^version:([ \t]+)("[^"]*"|'[^']*'|[^\s#"'|>][^\s#]*)([ \t]*#.*)?$`)

// SetVersion returns chartYAML with its version changed to version. Only that
// line changes: every other byte, comments and line endings included, stays as
// it is, so a diff shows nothing else. A trailing comment on the line is kept.
//
// It fails when Chart.yaml has no top-level version on one line, or has more
// than one, because then it cannot say which to change. It also fails if the
// result would not read back as the text of the version asked for: a version
// such as 123 would be a number.
func SetVersion(chartYAML []byte, version string) ([]byte, error) {
	lines := bytes.SplitAfter(chartYAML, []byte("\n"))

	found := -1
	for i, line := range lines {
		if !bytes.HasPrefix(line, []byte("version:")) {
			continue
		}
		if found >= 0 {
			return nil, errors.New("the Chart.yaml has more than one version")
		}
		found = i
	}
	if found < 0 {
		return nil, errors.New("the Chart.yaml has no version")
	}

	line, ending := splitEnding(lines[found])
	match := versionLine.FindSubmatch(line)
	if match == nil {
		return nil, fmt.Errorf("the version line of the Chart.yaml is not a value on one line: %q", line)
	}

	var edited bytes.Buffer
	for i, l := range lines {
		if i != found {
			edited.Write(l)
			continue
		}
		edited.WriteString("version:")
		edited.Write(match[1])
		edited.WriteString(version)
		edited.Write(match[3])
		edited.Write(ending)
	}

	var back struct {
		Version yaml.Node `yaml:"version"`
	}
	if err := yaml.Unmarshal(edited.Bytes(), &back); err != nil {
		return nil, fmt.Errorf("the Chart.yaml would not be valid YAML with version %q: %w", version, err)
	}
	if back.Version.Tag != "!!str" || back.Version.Value != version {
		return nil, fmt.Errorf("the Chart.yaml would not read %q as the text of the version", version)
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
