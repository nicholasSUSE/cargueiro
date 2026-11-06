package annotations

import (
	"fmt"
	"sort"
	"strings"
)

// valuesImage is an image found in values.yaml.
type valuesImage struct {
	// Path locates the image in values.yaml, such as "image.controller".
	Path string
	// Image is "repository:tag".
	Image string
}

// scanValues finds the images in values.yaml and reports the ones that are not
// written with a repository and a tag. An image is a map with a "repository"
// key. A map with an empty repository and an empty tag has no image configured
// and is skipped. Other keys, including one called "image" that holds a
// string, are not images and are ignored.
func scanValues(values map[string]any) ([]valuesImage, []string) {
	var scan valuesScan
	scan.walk(values, "")
	return scan.images, scan.problems
}

type valuesScan struct {
	images   []valuesImage
	problems []string
}

// walk visits node and everything under it, in key order.
func (s *valuesScan) walk(node any, path string) {
	switch n := node.(type) {
	case map[string]any:
		s.walkMap(n, path)
	case []any:
		for i, item := range n {
			s.walk(item, fmt.Sprintf("%s[%d]", path, i))
		}
	}
}

func (s *valuesScan) walkMap(m map[string]any, path string) {
	if _, isImage := m["repository"]; isImage {
		s.imageBlock(m, path)
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
		s.walk(m[key], childPath)
	}
}

// imageBlock reads a map that has a "repository" key.
func (s *valuesScan) imageBlock(block map[string]any, path string) {
	repository, tag := block["repository"], block["tag"]

	switch {
	case notString(repository) || notString(tag):
		s.problems = append(s.problems, valuesProblem(path, `"repository" and "tag" must be strings`))
	case blank(repository) && blank(tag):
		// No image configured.
	case blank(tag):
		s.problems = append(s.problems, valuesProblem(path, `"repository" is set but "tag" is empty`))
	case blank(repository):
		s.problems = append(s.problems, valuesProblem(path, `"tag" is set but "repository" is empty`))
	default:
		repositoryText, _ := repository.(string)
		tagText, _ := tag.(string)
		s.images = append(s.images, valuesImage{Path: path, Image: repositoryText + ":" + tagText})
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
