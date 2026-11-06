package annotations

import (
	"fmt"
	"strings"
)

// checkHidden reports a hidden annotation that is not "true". An
// absent or blank one is left to checkMissing.
func checkHidden(annotations map[string]string) []string {
	value := annotations[prefix+"hidden"]
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if err := hidden(value); err != nil {
		return []string{fmt.Sprintf("%shidden %q: %v", prefix, value, err)}
	}
	return nil
}

// hidden returns an error unless value is exactly "true": a CRD chart is never
// listed in Rancher.
func hidden(value string) error {
	if value != "true" {
		return fmt.Errorf("must be %q", "true")
	}
	return nil
}
