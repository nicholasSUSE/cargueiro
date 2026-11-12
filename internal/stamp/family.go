package stamp

import (
	"context"
	"fmt"

	"github.com/rancher/cargueiro/internal/rules"
)

// ChartPlan is what stamping one chart of a family would change.
type ChartPlan struct {
	Chart string
	Plan  *Plan
}

// FamilyPlan is what stamping a family would change. Building it writes nothing.
type FamilyPlan struct {
	// Version is the version of the family: the one of its main chart.
	Version string
	// Charts are the plans of the charts of the family, the main chart first.
	Charts []ChartPlan
}

// BuildFamily works out what stamping the family would change. dirs are the
// folders of the charts of the family, in the order of the family: the main chart
// first. The version of the main chart, the one given or the one worked out, is
// the version of the family.
//
// Only the main chart is stamped. It fails, and says why, before anything is
// written, for the same reasons Build does.
func BuildFamily(ctx context.Context, cfg *rules.Config, members rules.Family, dirs []string, opts Options) (*FamilyPlan, error) {
	if len(dirs) != len(members.Charts) {
		return nil, fmt.Errorf("the family %q has %d charts and %d folders were given", members.Name, len(members.Charts), len(dirs))
	}

	main := members.Main()
	plan, err := Build(ctx, cfg, dirs[0], opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", main.Name, err)
	}
	return &FamilyPlan{
		Version: plan.NewVersion,
		Charts:  []ChartPlan{{Chart: main.Name, Plan: plan}},
	}, nil
}

// Write writes the plan of every chart, in order.
func (p *FamilyPlan) Write() error {
	for _, chart := range p.Charts {
		if err := chart.Plan.Write(); err != nil {
			return fmt.Errorf("%s: %w", chart.Chart, err)
		}
	}
	return nil
}
