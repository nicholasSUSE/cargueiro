package oci

import (
	"fmt"
	"regexp"
	"strings"
)

// tagForm is the form OCI allows for a tag.
var tagForm = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

// ChartRef is a chart in a registry: <host>/<repository>:<tag>.
type ChartRef struct {
	// Host is the registry, with an optional port.
	Host string
	// Repository is the path of the chart in the registry, such as
	// "rancher/fleet".
	Repository string
	// Tag is the tag as the registry stores it: a "+" in the version is "_".
	Tag string
}

// String returns the reference without a scheme.
func (r ChartRef) String() string {
	return r.Host + "/" + r.Repository + ":" + r.Tag
}

// ParseChartRef parses "[oci://]host[:port]/path/chart:tag". A "+" in the tag is
// written "_" in the registry, because OCI tags cannot hold a "+", so a chart
// version can be given as it is in Chart.yaml. A reference without a tag, or
// pinned to a digest, is refused.
func ParseChartRef(s string) (ChartRef, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "oci://")

	switch {
	case s == "":
		return ChartRef{}, invalidRef("empty reference")
	case strings.Contains(s, "://"):
		return ChartRef{}, invalidRef("%q: only the oci:// scheme is accepted", s)
	case strings.Contains(s, "@"):
		return ChartRef{}, invalidRef("%q: references by digest (@sha256:...) are not supported yet, give a tag", s)
	case strings.ContainsAny(s, " \t\r\n?#"):
		return ChartRef{}, invalidRef("%q contains forbidden characters", s)
	}

	host, rest, found := strings.Cut(s, "/")
	if !found || rest == "" {
		return ChartRef{}, invalidRef("%q: want host/path/chart:tag", s)
	}
	if err := checkHost(host); err != nil {
		return ChartRef{}, invalidRef("%q: %v", s, err)
	}

	repository, tag, hasTag := cutLast(rest, ":")
	if !hasTag {
		return ChartRef{}, invalidRef("%q has no tag: want host/path/chart:tag", s)
	}
	if strings.Contains(repository, ":") {
		return ChartRef{}, invalidRef("%q: only the tag may follow a ':'", s)
	}
	for _, segment := range strings.Split(repository, "/") {
		if !repoName.MatchString(segment) {
			return ChartRef{}, invalidRef("%q: %q is not a valid repository name", s, segment)
		}
	}

	tag = strings.ReplaceAll(tag, "+", "_")
	if !tagForm.MatchString(tag) {
		return ChartRef{}, invalidRef("%q: %q is not a valid tag", s, tag)
	}

	return ChartRef{Host: host, Repository: repository, Tag: tag}, nil
}

// cutLast splits s around the last sep, like strings.Cut does around the first.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

func invalidRef(format string, args ...any) error {
	return fmt.Errorf("invalid chart reference: %s", fmt.Sprintf(format, args...))
}
