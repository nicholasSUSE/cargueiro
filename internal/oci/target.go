// Package oci pushes packaged Helm charts to OCI registries.
package oci

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidRegistry is returned when a registry address cannot be used.
var ErrInvalidRegistry = errors.New("invalid registry")

// Target is a registry location: a host (with optional port) and an optional
// repository path prefix under which charts are pushed.
type Target struct {
	Host string
	Path string
}

// ParseTarget parses "host[:port][/path]". A leading oci://, https:// or
// http:// scheme and trailing slashes are ignored.
func ParseTarget(s string) (Target, error) {
	s = strings.TrimSpace(s)
	for _, scheme := range []string{"oci://", "https://", "http://"} {
		s = strings.TrimPrefix(s, scheme)
	}
	s = strings.TrimRight(s, "/")

	if s == "" {
		return Target{}, invalid("empty address")
	}
	if strings.ContainsAny(s, " \t\r\n@?#") {
		return Target{}, invalid("%q contains forbidden characters", s)
	}

	host, path, _ := strings.Cut(s, "/")
	if err := checkHost(host); err != nil {
		return Target{}, invalid("%q: %v", s, err)
	}

	if path != "" {
		if strings.Contains(path, ":") {
			return Target{}, invalid("%q: path must not contain ':'", s)
		}
		for _, segment := range strings.Split(path, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return Target{}, invalid("%q has an invalid path", s)
			}
		}
	}

	return Target{Host: host, Path: path}, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRegistry, fmt.Sprintf(format, args...))
}

// checkHost accepts "name" or "name:port" with a numeric port.
func checkHost(host string) error {
	name, port, hasPort := strings.Cut(host, ":")
	if name == "" {
		return errors.New("no host")
	}
	if !hasPort {
		return nil
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("invalid port %q", port)
	}
	return nil
}

// Ref returns the reference a chart is pushed to: <host>[/<path>]/<name>:<version>.
func (t Target) Ref(name, version string) string {
	repo := t.Host
	if t.Path != "" {
		repo += "/" + t.Path
	}
	return repo + "/" + name + ":" + version
}
