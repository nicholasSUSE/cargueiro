package oci

import (
	"fmt"
	"strings"
)

// isProtectedHost reports whether name, already normalized, is a registry
// whose catalog repository is off limits.
func isProtectedHost(name string) bool {
	switch name {
	case "registry.suse.com", "stgregistry.suse.com":
		return true
	}
	return false
}

// protectedPrefix is the repository path, on a protected host, that holds
// published charts.
const protectedPrefix = "rancher/catalog"

// CheckTarget refuses t when it is, or lies under,
// <protected host>/rancher/catalog. It looks at the address only and makes no
// network call.
func CheckTarget(t Target) error {
	return checkRepository(t.Host, t.Path)
}

// checkRepository refuses host/repository when host is a protected registry
// and repository is rancher/catalog or lies under it.
func checkRepository(host, repository string) error {
	if !isProtectedHost(normalizeHost(host)) {
		return nil
	}

	repo := strings.ToLower(repository)
	if repo == protectedPrefix || strings.HasPrefix(repo, protectedPrefix+"/") {
		return fmt.Errorf("pushing to this registry is not allowed: %s/%s holds published charts that must never be overwritten",
			host, repository)
	}
	return nil
}

// normalizeHost lower-cases host and drops the port and a trailing dot, so
// that REGISTRY.suse.com., registry.suse.com:443 and registry.suse.com are
// all the same registry.
func normalizeHost(host string) string {
	name, _, _ := strings.Cut(host, ":")
	return strings.TrimSuffix(strings.ToLower(name), ".")
}
