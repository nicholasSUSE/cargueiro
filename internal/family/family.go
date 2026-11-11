// Package family puts what a command was given in the order of the family it
// belongs to.
package family

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/cargueiro/internal/rules"
)

// Resolve checks that items are exactly the charts of f, each once, and returns
// them in the order of the family: the main chart first. name gives the name of
// the chart an item holds, such as a folder or an archive.
//
// Every problem is reported, not only the first.
func Resolve[T any](f rules.Family, items []T, name func(T) (string, error)) ([]T, error) {
	byName := make(map[string]T, len(items))
	var problems []error
	named := true
	for _, item := range items {
		chart, err := name(item)
		if err != nil {
			problems = append(problems, err)
			named = false
			continue
		}
		switch {
		case !slices.Contains(f.Names(), chart):
			problems = append(problems, fmt.Errorf("chart %q is not in the family %q", chart, f.Name))
		default:
			if _, dup := byName[chart]; dup {
				problems = append(problems, fmt.Errorf("chart %q is given twice", chart))
				continue
			}
			byName[chart] = item
		}
	}

	// A chart that could not be named is not missing: its error says why.
	if named {
		var missing []string
		for _, chart := range f.Charts {
			if _, ok := byName[chart.Name]; !ok {
				missing = append(missing, chart.Name)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Errorf("the family %q is missing %s", f.Name, strings.Join(missing, ", ")))
		}
	}

	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	ordered := make([]T, 0, len(f.Charts))
	for _, chart := range f.Charts {
		ordered = append(ordered, byName[chart.Name])
	}
	return ordered, nil
}
