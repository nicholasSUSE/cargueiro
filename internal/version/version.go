// Package version exposes build information injected at link time.
package version

import "fmt"

// These values are set via -ldflags at build time, which requires
// package-level variables.
var (
	version = "dev"
	commit  = "none"
)

// Info describes the running build.
type Info struct {
	Version string
	Commit  string
}

// Get returns the build information of the running binary.
func Get() Info {
	return Info{Version: version, Commit: commit}
}

// String formats the build information for display.
func (i Info) String() string {
	return fmt.Sprintf("%s (commit %s)", i.Version, i.Commit)
}
