package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// majorOf reads the major at the start of a chart version.
	majorOf = regexp.MustCompile(`^([0-9]+)\.`)
	// rancherLowerBound reads the Rancher version of the lower bound of a
	// rancher-version annotation: 2.16 in ">= 2.16.0-0 < 2.17.0-0".
	rancherLowerBound = regexp.MustCompile(`^>=\s*(2\.[0-9]+)\.[0-9]+`)
)

// CheckMajor returns an error unless the major of version is the one the
// Policy ties to the Rancher version in the lower bound of rancherVersion, the
// value of the catalog.cattle.io/rancher-version annotation.
func (p *Policy) CheckMajor(version, rancherVersion string) error {
	match := majorOf.FindStringSubmatch(version)
	if match == nil {
		return fmt.Errorf("version %q does not start with a major", version)
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return fmt.Errorf("version %q has a major that is too large", version)
	}

	bound := rancherLowerBound.FindStringSubmatch(strings.TrimSpace(rancherVersion))
	if bound == nil {
		return fmt.Errorf("rancher-version %q has no lower bound like >= 2.16.0-0", rancherVersion)
	}
	rancher := bound[1]

	want, ok := p.majors[rancher]
	if !ok {
		return fmt.Errorf("no chart major for Rancher %s in the rules file", rancher)
	}
	if major != want {
		return fmt.Errorf("version %q has major %d, but charts for Rancher %s have major %d", version, major, rancher, want)
	}
	return nil
}
