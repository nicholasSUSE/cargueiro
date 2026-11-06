package annotations

import "strings"

// checkDependenciesMatch compares the images in values.yaml with the helm.sh/images
// annotation:
//   - every image in values.yaml must be listed in the annotation;
//   - every well-formed annotation entry that is not in values.yaml must name
//     the subchart it belongs to in "dependency".
//
// An annotation that cannot be read is left to
// checkDependenciesAnnotationStandard, which reports it.
func checkDependenciesMatch(annotations map[string]string, images []valuesImage) []string {
	var entries []helmImageEntry
	if value := annotations[imageDependenciesAnnotation]; strings.TrimSpace(value) != "" {
		var err error
		if entries, err = parseHelmImages(value); err != nil {
			return nil
		}
	}

	listed := make(map[string]bool, len(entries))
	for _, entry := range entries {
		listed[strings.TrimSpace(entry.Image)] = true
	}
	inValues := make(map[string]bool, len(images))
	for _, image := range images {
		inValues[image.Image] = true
	}

	var problems []string

	reported := make(map[string]bool)
	for _, image := range images {
		if listed[image.Image] || reported[image.Image] {
			continue
		}
		reported[image.Image] = true
		problems = append(problems, valuesProblem(image.Path, "image %q is not listed in %s", image.Image, imageDependenciesAnnotation))
	}

	for i, entry := range entries {
		image := strings.TrimSpace(entry.Image)
		if helmImage(image) != nil || inValues[image] || strings.TrimSpace(entry.Dependency) != "" {
			continue
		}
		problems = append(problems, entryProblem(i+1, "image %q is not in values.yaml and has no dependency", image))
	}
	return problems
}
