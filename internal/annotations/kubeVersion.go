package annotations

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// checkKubeVersion reports a kube-version annotation that is not a bounded range of
// Kubernetes versions. An absent or blank one is left to checkMissing.
func checkKubeVersion(annotations map[string]string) []string {
	value := annotations[prefix+"kube-version"]
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if err := kubeVersion(value); err != nil {
		return []string{fmt.Sprintf("%skube-version %q: %v", prefix, value, err)}
	}
	return nil
}

// kubeVersionRange matches a range with both bounds: ">= 1.34.0-0 < 1.37.0-0". The
// -0 suffix is required, because Rancher compares ranges including
// prereleases.
var kubeVersionRange = regexp.MustCompile(`^>=\s*(\d+)\.(\d+)\.(\d+)-0\s*<\s*(\d+)\.(\d+)\.(\d+)-0$`)

// kubeVersion returns an error unless value is a range with a lower and
// an upper bound, the lower below the upper.
func kubeVersion(value string) error {
	m := kubeVersionRange.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return fmt.Errorf("must look like %q", ">= 1.34.0-0 < 1.37.0-0")
	}

	lower, err := kubeVersionNumbers(m[1:4])
	if err != nil {
		return err
	}
	upper, err := kubeVersionNumbers(m[4:7])
	if err != nil {
		return err
	}
	if !kubeLess(lower, upper) {
		return fmt.Errorf("lower bound %d.%d.%d-0 must be below upper bound %d.%d.%d-0",
			lower[0], lower[1], lower[2], upper[0], upper[1], upper[2])
	}
	return nil
}

// kubeVersionNumbers reads the major, minor and patch numbers of a version.
func kubeVersionNumbers(parts []string) ([3]int, error) {
	var numbers [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return numbers, fmt.Errorf("version number %q is too large", part)
		}
		numbers[i] = n
	}
	return numbers, nil
}

// kubeLess reports whether Kubernetes version a is below b. Versions are
// {major, minor, patch}.
func kubeLess(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
