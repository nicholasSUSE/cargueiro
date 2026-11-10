package index

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
)

// Ref names one version of one chart.
type Ref struct {
	Chart   string
	Version string
}

// State is what a charts repository holds about its versions, seen from the
// three places that must agree.
type State struct {
	// Entries are the versions that have an entry file.
	Entries []Ref
	// Published are the versions listed in the index.yaml that is committed.
	Published []Ref
	// Archives are the versions that have a .tgz in assets/.
	Archives []Ref
}

// Added returns the versions to publish: the ones with an entry that are not in
// the index yet, sorted by chart and then by version.
//
// A version is never removed once it is in the index, and the index and assets/
// never hold what the entries do not, so Added fails, naming every version at
// fault, when the three places disagree in any other way.
func (s State) Added() ([]Ref, error) {
	entries, published, archives := refSet(s.Entries), refSet(s.Published), refSet(s.Archives)

	all := slices.Concat(s.Entries, s.Published, s.Archives)
	slices.SortFunc(all, compareRefs)
	all = slices.Compact(all)

	var added []Ref
	var problems []error
	for _, ref := range all {
		_, entry := entries[ref]
		_, indexed := published[ref]
		_, archived := archives[ref]

		switch {
		case indexed && !entry:
			problems = append(problems, fmt.Errorf("%s %s is in index.yaml but has no entry: entries are never removed", ref.Chart, ref.Version))
		case archived && !entry:
			problems = append(problems, fmt.Errorf("assets/ has an archive of %s %s but there is no entry for it", ref.Chart, ref.Version))
		case indexed && !archived:
			problems = append(problems, fmt.Errorf("%s %s is in index.yaml but has no archive in assets/", ref.Chart, ref.Version))
		case archived && !indexed:
			problems = append(problems, fmt.Errorf("%s %s is not in index.yaml yet but already has an archive in assets/", ref.Chart, ref.Version))
		case !indexed:
			added = append(added, ref)
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return added, nil
}

func compareRefs(a, b Ref) int {
	return cmp.Or(cmp.Compare(a.Chart, b.Chart), cmp.Compare(a.Version, b.Version))
}

func refSet(refs []Ref) map[Ref]struct{} {
	set := make(map[Ref]struct{}, len(refs))
	for _, ref := range refs {
		set[ref] = struct{}{}
	}
	return set
}
