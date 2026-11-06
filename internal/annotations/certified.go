package annotations

import "errors"

// checkCertified reports a certified annotation that is not "rancher". An
// absent or blank one is left to checkMissing.
func checkCertified(annotations map[string]string) []string {
	return checkValue(annotations, "certified", certified)
}

// certified returns an error unless value is exactly "rancher".
func certified(value string) error {
	if value != "rancher" {
		return errors.New(`must be "rancher"`)
	}
	return nil
}
