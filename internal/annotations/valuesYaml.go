package annotations

import (
	"fmt"
	"sort"
	"strings"
)

// checkValuesYaml reports the images in values.yaml that are not written with
// a repository and a tag. An image is a map with a "repository" key. A map
// with an empty repository and an empty tag has no image configured and is
// skipped.
func checkValuesYaml(values map[string]any) []string {
	return valuesProblems(values, "")
}

// valuesProblems walks node and reports the problems under path.
func valuesProblems(node any, path string) []string {
	switch n := node.(type) {
	case map[string]any:
		return mapProblems(n, path)
	case []any:
		var problems []string
		for i, item := range n {
			problems = append(problems, valuesProblems(item, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return problems
	default:
		return nil
	}
}

func mapProblems(m map[string]any, path string) []string {
	var problems []string
	if _, isImage := m["repository"]; isImage {
		problems = append(problems, imageBlockProblems(m, path)...)
	}

	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		childPath := key
		if path != "" {
			childPath = path + "." + key
		}

		if flat, isString := m[key].(string); isString && key == "image" && strings.TrimSpace(flat) != "" {
			problems = append(problems, valuesProblem(childPath, `image %q must be written with "repository" and "tag" keys`, flat))
		}
		problems = append(problems, valuesProblems(m[key], childPath)...)
	}
	return problems
}

// imageBlockProblems checks a map that has a "repository" key.
func imageBlockProblems(block map[string]any, path string) []string {
	repository, tag := block["repository"], block["tag"]

	switch {
	case notString(repository) || notString(tag):
		return []string{valuesProblem(path, `"repository" and "tag" must be strings`)}
	case blank(repository) && blank(tag):
		return nil
	case blank(tag):
		return []string{valuesProblem(path, `"repository" is set but "tag" is empty`)}
	case blank(repository):
		return []string{valuesProblem(path, `"tag" is set but "repository" is empty`)}
	default:
		return nil
	}
}

func valuesProblem(path, format string, args ...any) string {
	if path == "" {
		path = "."
	}
	return fmt.Sprintf("values.yaml %s: %s", path, fmt.Sprintf(format, args...))
}

// notString reports a value that is set but is not text, such as a number.
func notString(v any) bool {
	_, isString := v.(string)
	return v != nil && !isString
}

// blank reports a missing, null or empty value.
func blank(v any) bool {
	s, _ := v.(string)
	return strings.TrimSpace(s) == ""
}
