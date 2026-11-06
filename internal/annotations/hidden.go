package annotations

import "errors"

// checkHidden reports a hidden annotation that is not "true". An absent or
// blank one is left to checkMissing. Only CRD charts are checked (see
// ValidateCRD).
func checkHidden(annotations map[string]string) []string {
	return checkValue(annotations, "hidden", hidden)
}

// hidden returns an error unless value is exactly "true": a CRD chart is never
// listed in Rancher.
func hidden(value string) error {
	if value != "true" {
		return errors.New(`must be "true"`)
	}
	return nil
}
