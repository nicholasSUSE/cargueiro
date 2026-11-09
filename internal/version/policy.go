package version

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
)

// number is a version number without leading zeros.
const number = `(0|[1-9][0-9]*)`

// plainVersion is the form of a chart version without a +up part: 1XY.Z.W.
var plainVersion = regexp.MustCompile(`^1[0-9]{2}\.` + number + `\.` + number + `$`)

// Policy is the form a Rancher chart version must have.
type Policy struct {
	standard *regexp.Regexp
	// forms are the allowed prereleases as written in an error: -rc.N or -beta.N.
	forms string
	// majors maps a Rancher version, such as 2.16, to the major its charts carry.
	majors map[string]int
}

// NewPolicy returns the Policy that allows the given prerelease names after the
// +up part, as in the rc of -rc.1, and ties each Rancher version, such as 2.16,
// to the major its charts carry, such as 111.
func NewPolicy(prereleases []string, majors map[string]int) (*Policy, error) {
	if len(prereleases) == 0 {
		return nil, errors.New("version policy: no prereleases")
	}
	if len(majors) == 0 {
		return nil, errors.New("version policy: no majors")
	}

	names := make([]string, len(prereleases))
	forms := make([]string, len(prereleases))
	for i, name := range prereleases {
		names[i] = regexp.QuoteMeta(name)
		forms[i] = "-" + name + ".N"
	}

	pattern := `^1[0-9]{2}\.` + number + `\.` + number +
		`\+up` + number + `\.` + number + `\.` + number +
		`(-(` + strings.Join(names, "|") + `)\.` + number + `)?$`
	return &Policy{
		standard: regexp.MustCompile(pattern),
		forms:    joinOr(forms),
		majors:   maps.Clone(majors),
	}, nil
}

// Validate returns an error unless version has the form of a Rancher chart
// version: 1XY.Z.W+upA.B.C, with an optional prerelease after the +up part. A
// plain chart has no +up part and no prerelease: 1XY.Z.W.
func (p *Policy) Validate(version string, plain bool) error {
	if plain {
		if !plainVersion.MatchString(version) {
			return fmt.Errorf("version %q must look like 1XY.Z.W", version)
		}
		return nil
	}

	if !p.standard.MatchString(version) {
		return fmt.Errorf("version %q must look like 1XY.Z.W+upA.B.C, optionally followed by %s", version, p.forms)
	}
	return nil
}

// joinOr writes forms as "a, b or c".
func joinOr(forms []string) string {
	if len(forms) == 1 {
		return forms[0]
	}
	return strings.Join(forms[:len(forms)-1], ", ") + " or " + forms[len(forms)-1]
}
