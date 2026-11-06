package annotations

import (
	"fmt"
	"strings"
)

// checkPermitsOS reports a permits-os annotation that is not a valid list of
// operating systems. An absent or blank one is left to checkMissing.
func checkPermitsOS(annotations map[string]string) []string {
	return checkValue(annotations, "permits-os", permitsOS)
}

// permitsOS returns an error unless value is a comma-separated list, without
// spaces, of "linux" and "windows" with no repeats, and "linux" among them.
func permitsOS(value string) error {
	seen := make(map[string]bool)
	for _, name := range strings.Split(value, ",") {
		if name != "linux" && name != "windows" {
			return fmt.Errorf("must list %q, and optionally %q, separated by commas", "linux", "windows")
		}
		if seen[name] {
			return fmt.Errorf("%q is listed twice", name)
		}
		seen[name] = true
	}
	if !seen["linux"] {
		return fmt.Errorf("must include %q", "linux")
	}
	return nil
}
