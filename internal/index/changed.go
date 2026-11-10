package index

import (
	"bytes"
	"fmt"
	"slices"

	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"
)

// changedRefs returns the versions of published whose entry in regenerated is
// not exactly the same, sorted by chart and then by version. A published version
// never changes, so any of them is an error.
//
// A version that regenerated does not have is not reported: State.Added does.
func changedRefs(published, regenerated *repo.IndexFile) ([]Ref, error) {
	if published == nil {
		return nil, nil
	}

	now := make(map[Ref]*repo.ChartVersion)
	for name, versions := range regenerated.Entries {
		for _, v := range versions {
			now[Ref{Chart: name, Version: v.Version}] = v
		}
	}

	var changed []Ref
	for name, versions := range published.Entries {
		for _, before := range versions {
			ref := Ref{Chart: name, Version: before.Version}
			after, found := now[ref]
			if !found {
				continue
			}
			same, err := sameEntry(before, after)
			if err != nil {
				return nil, fmt.Errorf("compare %s %s: %w", ref.Chart, ref.Version, err)
			}
			if !same {
				changed = append(changed, ref)
			}
		}
	}
	slices.SortFunc(changed, compareRefs)
	return changed, nil
}

// sameEntry reports whether a and b are written the same in an index.yaml.
func sameEntry(a, b *repo.ChartVersion) (bool, error) {
	x, err := yaml.Marshal(a)
	if err != nil {
		return false, err
	}
	y, err := yaml.Marshal(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(x, y), nil
}
