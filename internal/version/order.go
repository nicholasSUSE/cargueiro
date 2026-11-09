package version

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"github.com/Masterminds/semver/v3"
)

// parsed is a chart version taken apart to be compared.
type parsed struct {
	version string
	// base is 1XY.Z.W.
	base *semver.Version
	// upstream is A.B.C. It is nil for a plain chart.
	upstream *semver.Version
	// rank is the place of the prerelease name in the order the Policy was
	// given, and number is the N of -name.N. hasPrerelease tells whether the
	// version has a prerelease at all.
	hasPrerelease bool
	rank          int
	number        int
}

// SortNewestFirst returns versions with the newest first. It fails, naming the
// version, when one does not follow the Rancher chart version rules, because
// then there is nothing to order it by. The versions given are not changed.
//
// Versions are compared by 1XY.Z.W, then by the upstream A.B.C after +up, which
// can be the same for two versions when a chart owner made a mistake, then by
// the prerelease. A version without a prerelease is newer than any prerelease
// of the same A.B.C. Prereleases are compared by the rank of their name, which
// is the order the Policy was given, and then by their number as a number.
func (p *Policy) SortNewestFirst(versions []string, plain bool) ([]string, error) {
	items := make([]parsed, 0, len(versions))
	for _, v := range versions {
		item, err := p.parseVersion(v, plain)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	slices.SortStableFunc(items, func(a, b parsed) int {
		return compare(b, a)
	})

	sorted := make([]string, len(items))
	for i, item := range items {
		sorted[i] = item.version
	}
	return sorted, nil
}

// parseVersion validates version and takes it apart.
func (p *Policy) parseVersion(version string, plain bool) (parsed, error) {
	if err := p.Validate(version, plain); err != nil {
		return parsed{}, err
	}

	if plain {
		base, err := semver.StrictNewVersion(version)
		if err != nil {
			return parsed{}, fmt.Errorf("version %q: %w", version, err)
		}
		return parsed{version: version, base: base}, nil
	}

	match := p.parse.FindStringSubmatch(version)
	if match == nil {
		// Validate accepted it, so the two patterns disagree.
		return parsed{}, fmt.Errorf("version %q cannot be taken apart", version)
	}
	base, err := semver.StrictNewVersion(match[1])
	if err != nil {
		return parsed{}, fmt.Errorf("version %q: %w", version, err)
	}
	upstream, err := semver.StrictNewVersion(match[2])
	if err != nil {
		return parsed{}, fmt.Errorf("version %q: %w", version, err)
	}

	item := parsed{version: version, base: base, upstream: upstream}
	if match[3] != "" {
		number, err := strconv.Atoi(match[4])
		if err != nil {
			return parsed{}, fmt.Errorf("version %q has a prerelease number that is too large", version)
		}
		item.hasPrerelease = true
		item.rank = p.rank[match[3]]
		item.number = number
	}
	return item, nil
}

// compare orders two versions: positive when a is newer than b.
func compare(a, b parsed) int {
	if c := a.base.Compare(b.base); c != 0 {
		return c
	}
	if a.upstream != nil && b.upstream != nil {
		if c := a.upstream.Compare(b.upstream); c != 0 {
			return c
		}
	}

	switch {
	case !a.hasPrerelease && !b.hasPrerelease:
		return 0
	case !a.hasPrerelease:
		return 1
	case !b.hasPrerelease:
		return -1
	}
	// A lower rank is a newer prerelease.
	if c := cmp.Compare(b.rank, a.rank); c != 0 {
		return c
	}
	return cmp.Compare(a.number, b.number)
}
