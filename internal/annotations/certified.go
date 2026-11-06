package annotations

import (
	"fmt"
	"strings"
)

// checkCertified reports a certified annotation that is not "rancher". An
// absent or blank one is left to checkMissing.
func checkCertified(annotations map[string]string) []string {
	value := annotations[prefix+"certified"]
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if err := certified(value); err != nil {
		return []string{fmt.Sprintf("%scertified %q: %v", prefix, value, err)}
	}
	return nil
}

// certified returns an error unless value is exactly "rancher".
func certified(value string) error {
	if value != "rancher" {
		return fmt.Errorf("must be %q", "rancher")
	}
	return nil
}
