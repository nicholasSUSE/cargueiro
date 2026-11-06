package annotations

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// imageDependenciesAnnotation is the annotation Helm HIP-15 defines for the images a chart uses.
const imageDependenciesAnnotation = "helm.sh/images"

// ValidateHelmImages checks the helm.sh/images annotation of a chart and the
// images in its values.yaml, which every chart kind runs. values is the parsed
// values.yaml.
func ValidateHelmImages(annotations map[string]string, values map[string]any) []string {
	images, valuesProblems := scanValues(values)
	return collect(
		checkDependenciesAnnotationStandard(annotations),
		valuesProblems,
		checkDependenciesMatch(annotations, images),
	)
}

// checkDependenciesAnnotationStandard reports an annotation that is not a list of entries
// with a unique name and an image written repository:tag. An absent or blank
// annotation is left to checkDependenciesMatch, which compares it with values.yaml.
//
// HIP-15 asks for a list of entries with a name and an image. Rancher policy
// adds that names are unique (HIP-15 allows repeats, as in its Kafka example)
// and that every image has a tag (HIP-15 only says "a location").
func checkDependenciesAnnotationStandard(annotations map[string]string) []string {
	value := annotations[imageDependenciesAnnotation]
	if strings.TrimSpace(value) == "" {
		return nil
	}

	entries, err := parseHelmImages(value)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", imageDependenciesAnnotation, err)}
	}

	problems, _ := checkEntries(entries)
	return problems
}

// checkEntries reports the problems in entries and which entries have one.
func checkEntries(entries []helmImageEntry) (problems []string, malformed []bool) {
	malformed = make([]bool, len(entries))
	firstUse := make(map[string]int)
	for i, entry := range entries {
		n := i + 1
		problemsBefore := len(problems)
		name, image := strings.TrimSpace(entry.Name), strings.TrimSpace(entry.Image)

		first, nameUsed := firstUse[name]
		switch {
		case name == "":
			problems = append(problems, entryProblem(n, "name must be a non-empty string"))
		case nameUsed:
			// Rancher policy: HIP-15 allows the same name more than once.
			problems = append(problems, entryProblem(n, "name %q is already used by entry %d", name, first))
		default:
			firstUse[name] = n
		}

		imageErr := helmImage(image)
		switch {
		case image == "":
			problems = append(problems, entryProblem(n, "image must be a non-empty string"))
		case imageErr != nil:
			problems = append(problems, entryProblem(n, "image %q %v", image, imageErr))
		}

		malformed[i] = len(problems) > problemsBefore
	}
	return problems, malformed
}

// entryProblem describes a problem with entry number n of the annotation.
func entryProblem(n int, format string, args ...any) string {
	return fmt.Sprintf("%s entry %d: %s", imageDependenciesAnnotation, n, fmt.Sprintf(format, args...))
}

// helmImageEntry is one element of the helm.sh/images list. Other properties
// are allowed by HIP-15 and ignored here.
type helmImageEntry struct {
	Name       string `yaml:"name"`
	Image      string `yaml:"image"`
	Dependency string `yaml:"dependency"`
}

// parseHelmImages reads the annotation value as a list of entries.
func parseHelmImages(value string) ([]helmImageEntry, error) {
	errNotAList := errors.New("must be a YAML list of entries with a name and an image")

	var entries []helmImageEntry
	if err := yaml.Unmarshal([]byte(value), &entries); err != nil {
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			return nil, errNotAList
		}
		return nil, fmt.Errorf("%w: %w", errNotAList, err)
	}
	if entries == nil {
		return nil, errNotAList
	}
	return entries, nil
}

// helmImage returns an error unless image is written repository:tag, with a
// tag and no empty part between slashes in the repository. The registry is
// not looked at. Requiring the tag is Rancher policy.
func helmImage(image string) error {
	if strings.ContainsAny(image, " \t\r\n") {
		return fmt.Errorf(`must look like "repository:tag" without spaces`)
	}
	colon := strings.LastIndex(image, ":")
	if colon <= 0 || colon < strings.LastIndex(image, "/") || colon == len(image)-1 {
		return fmt.Errorf(`must look like "repository:tag"`)
	}

	repository := image[:colon]
	if strings.HasPrefix(repository, "/") || strings.HasSuffix(repository, "/") || strings.Contains(repository, "//") {
		return fmt.Errorf(`must look like "repository:tag" with no empty part in the repository`)
	}
	return nil
}
