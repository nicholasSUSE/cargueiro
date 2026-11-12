package stamp

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/rancher/cargueiro/internal/chartversion"
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

	// undo puts back the files that were written when a later write fails.
	undo func(written []*Plan) error
}

// BuildFamily works out what stamping the family would change. dirs are the
// folders of the charts of the family, in the order of the family: the main chart
// first.
//
// The version of the main chart, the one given or the one worked out from its own
// registry, is the version of the family, and every other chart takes it. When it
// is worked out, the registries of the other charts are read as well, only to
// check that the latest version each of them has is the one the main chart has.
// When it is given, nothing is read.
//
// It fails, and says why, before anything is written, for the same reasons Build
// does, and reports the problem of every chart.
func BuildFamily(ctx context.Context, cfg *rules.Config, members rules.Family, dirs []string, opts Options) (*FamilyPlan, error) {
	if len(dirs) != len(members.Charts) {
		return nil, fmt.Errorf("the family %q has %d charts and %d folders were given", members.Name, len(members.Charts), len(dirs))
	}

	main := members.Main()
	mainPlan, err := Build(ctx, cfg, dirs[0], opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", main.Name, err)
	}

	var problems []error
	if opts.Version == "" {
		if err := chartversion.CheckReleased(ctx, cfg, members, mainPlan.Released, opts.versionOptions()); err != nil {
			problems = append(problems, err)
		}
	}

	plans := []ChartPlan{{Chart: main.Name, Plan: mainPlan}}
	others := opts
	others.Version = mainPlan.NewVersion
	for i, chart := range members.Charts[1:] {
		plan, err := Build(ctx, cfg, dirs[i+1], others)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", chart.Name, err))
			continue
		}
		plans = append(plans, ChartPlan{Chart: chart.Name, Plan: plan})
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	return &FamilyPlan{Version: mainPlan.NewVersion, Charts: plans, undo: restoreAll}, nil
}

// Write writes the plan of every chart, in order. When a write fails, the files
// already written are put back as they were. If that fails too, the error says
// that the charts are half stamped.
func (p *FamilyPlan) Write() error {
	var written []*Plan
	for _, chart := range p.Charts {
		if err := chart.Plan.Write(); err != nil {
			err = fmt.Errorf("%s: %w", chart.Chart, err)
			if undoErr := p.undo(written); undoErr != nil {
				return fmt.Errorf("%w: and the old files could not be put back (%v): something catastrophic happened, "+
					"the charts are half stamped and the git tree is dirty: discard the changes (git reset --hard) before running it again",
					err, undoErr)
			}
			return fmt.Errorf("%w: the charts that were already written were put back as they were", err)
		}
		if chart.Plan.Changed() {
			written = append(written, chart.Plan)
		}
	}
	return nil
}

// restoreAll puts back the Chart.yaml of every plan that was written, the last
// one first, and reports every one that could not be put back.
func restoreAll(written []*Plan) error {
	var problems []error
	for _, plan := range slices.Backward(written) {
		if err := plan.restore(); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}
