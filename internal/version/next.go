package version

import (
	"errors"
	"fmt"
	"slices"
)

// Next returns the version of the next release of a chart, for the Rancher
// major major, whose upstream version is upstream (A.B.C, optionally with a
// prerelease such as -rc.1). published are the versions already released, of
// every major: the ones of other majors are left out.
//
// The release counter Z.W is worked out from what is published:
//   - a prerelease of the same upstream A.B.C is published and no final one:
//     the new version keeps its Z.W, so the candidates and the final of one
//     upstream version share it;
//   - otherwise W is the highest published W plus one. There is no bump type to
//     work out and no roll-over;
//   - nothing is published: 0.0.
//
// A plain chart has no upstream part: its version is 1XY.Z.(W+1), and upstream
// is not used.
//
// It fails when a published version does not follow the rules, because then it
// cannot be ordered, when upstream does not make a valid version, and when the
// version it would give is already published.
func (p *Policy) Next(published []string, major int, upstream string, plain bool) (string, error) {
	var items []parsed
	for _, v := range published {
		item, err := p.parseVersion(v, plain)
		if err != nil {
			return "", err
		}
		if int(item.base.Major()) == major {
			items = append(items, item)
		}
	}
	slices.SortStableFunc(items, func(a, b parsed) int { return compare(b, a) })

	if plain {
		if len(items) == 0 {
			return fmt.Sprintf("%d.0.0", major), nil
		}
		return fmt.Sprintf("%d.%d.%d", major, items[0].base.Minor(), items[0].base.Patch()+1), nil
	}

	next, err := p.parseVersion(fmt.Sprintf("%d.0.0+up%s", major, upstream), false)
	if err != nil {
		return "", errors.New(upstreamError(upstream, err))
	}

	var sameUpstream []parsed
	final := false
	for _, item := range items {
		if item.upstream.Equal(next.upstream) {
			sameUpstream = append(sameUpstream, item)
			final = final || !item.hasPrerelease
		}
	}

	var z, w uint64
	switch {
	case len(sameUpstream) > 0 && !final:
		z, w = sameUpstream[0].base.Minor(), sameUpstream[0].base.Patch()
	case len(items) > 0:
		z, w = items[0].base.Minor(), items[0].base.Patch()+1
	}

	version := fmt.Sprintf("%d.%d.%d+up%s", major, z, w, upstream)
	for _, item := range items {
		if item.version == version {
			return "", fmt.Errorf("version %s is already published", version)
		}
	}
	return version, nil
}

// upstreamError explains that the upstream version cannot be part of a version.
func upstreamError(upstream string, err error) string {
	return fmt.Sprintf("the upstream version %q does not make a valid version: %v", upstream, err)
}
