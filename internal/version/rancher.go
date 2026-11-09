package version

import (
	"strconv"
	"strings"
)

// MinRancherMinor is the lowest Rancher 2.x minor version that charts are made
// for in the new system: 2.16. Rancher 2.15 and before are the old system.
const MinRancherMinor = 16

// SupportedRancher reports whether a Rancher version written as 2.N, such as
// the one RancherOf returns, is one the new system supports.
func SupportedRancher(rancher string) bool {
	major, minor, found := strings.Cut(rancher, ".")
	if !found || major != "2" {
		return false
	}
	n, err := strconv.Atoi(minor)
	return err == nil && n >= MinRancherMinor
}
