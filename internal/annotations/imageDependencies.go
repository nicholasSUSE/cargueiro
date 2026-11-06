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
	return collect(
		checkDependenciesAnnotationStandard(annotations),
		checkValuesYaml(values),
	)
}

// checkDependenciesAnnotationStandard reports an annotation that is not a list of entries
// with a unique name and an image written repository:tag. An absent or blank
// annotation is left to the check that compares it with values.yaml.
func checkDependenciesAnnotationStandard(annotations map[string]string) []string {
	value := annotations[imageDependenciesAnnotation]
	if strings.TrimSpace(value) == "" {
		return nil
	}

	entries, err := parseHelmImages(value)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", imageDependenciesAnnotation, err)}
	}

	var problems []string
	firstUse := make(map[string]int)
	for i, entry := range entries {
		n := i + 1
		name, image := strings.TrimSpace(entry.Name), strings.TrimSpace(entry.Image)

		first, nameUsed := firstUse[name]
		switch {
		case name == "":
			problems = append(problems, fmt.Sprintf("%s entry %d: name must be a non-empty string", imageDependenciesAnnotation, n))
		case nameUsed:
			problems = append(problems, fmt.Sprintf("%s entry %d: name %q is already used by entry %d", imageDependenciesAnnotation, n, name, first))
		default:
			firstUse[name] = n
		}

		imageErr := helmImage(image)
		switch {
		case image == "":
			problems = append(problems, fmt.Sprintf("%s entry %d: image must be a non-empty string", imageDependenciesAnnotation, n))
		case imageErr != nil:
			problems = append(problems, fmt.Sprintf("%s entry %d: image %q %v", imageDependenciesAnnotation, n, image, imageErr))
		}
	}
	return problems
}

// helmImageEntry is one element of the helm.sh/images list. Other properties
// are allowed by HIP-15 and ignored here.
type helmImageEntry struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
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
// tag. The registry is not looked at.
func helmImage(image string) error {
	if strings.ContainsAny(image, " \t\r\n") {
		return fmt.Errorf(`must look like "repository:tag" without spaces`)
	}
	colon := strings.LastIndex(image, ":")
	if colon <= 0 || colon < strings.LastIndex(image, "/") || colon == len(image)-1 {
		return fmt.Errorf(`must look like "repository:tag"`)
	}
	return nil
}
