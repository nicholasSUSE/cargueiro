// Package version holds what cargueiro knows about versions: its own, injected at
// link time, and how a chart version is written as an OCI tag.
package version

// version is set via -ldflags at build time, which requires a package-level
// variable. A build that the release workflow did not make keeps "dev".
var version = "dev" //nolint:gochecknoglobals // set via ldflags

// Get returns the version of the running binary: the release tag, or "dev".
func Get() string {
	return version
}
