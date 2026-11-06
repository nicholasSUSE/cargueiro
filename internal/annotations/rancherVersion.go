package annotations

import (
	"fmt"
	"regexp"
	"strconv"
)

// checkRancherVersion reports a rancher-version annotation that is not a
// bounded range of Rancher versions. An absent or blank one is left to
// checkMissing.
func checkRancherVersion(annotations map[string]string) []string {
	return checkValue(annotations, "rancher-version", rancherVersion)
}

// rancherVersionRange matches a range with both bounds: ">= 2.15.0-0 < 2.16.0-0".
// The -0 suffix is required, because Rancher compares ranges including
// prereleases.
var rancherVersionRange = regexp.MustCompile(`^>=\s*(\d+)\.(\d+)\.(\d+)-0\s*<\s*(\d+)\.(\d+)\.(\d+)-0$`)

// rancherVersion returns an error unless value is a range with a lower and an
// upper bound, the lower below the upper.
func rancherVersion(value string) error {
	m := rancherVersionRange.FindStringSubmatch(value)
	if m == nil {
		return fmt.Errorf("must look like %q", ">= 2.15.0-0 < 2.16.0-0")
	}

	lower, err := rancherVersionNumbers(m[1:4])
	if err != nil {
		return err
	}
	upper, err := rancherVersionNumbers(m[4:7])
	if err != nil {
		return err
	}
	if !rancherLess(lower, upper) {
		return fmt.Errorf("lower bound %d.%d.%d-0 must be below upper bound %d.%d.%d-0",
			lower[0], lower[1], lower[2], upper[0], upper[1], upper[2])
	}
	return nil
}

// rancherVersionNumbers reads the major, minor and patch numbers of a version.
func rancherVersionNumbers(parts []string) ([3]int, error) {
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

// rancherLess reports whether Rancher version a is below b. Versions are
// {major, minor, patch}.
func rancherLess(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
