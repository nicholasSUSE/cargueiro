package stamp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/version"
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
		if err := checkReleased(ctx, cfg, members, mainPlan.Released, opts); err != nil {
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

// checkReleased fails unless the latest version released by each chart of the
// family is the one the main chart has, whose released versions are given. A
// chart with no repository yet has no versions. The way out is to give the
// version by hand.
func checkReleased(ctx context.Context, cfg *rules.Config, members rules.Family, mainReleased []string, opts Options) error {
	versions := cfg.Versions()
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return err
	}
	latest := func(chart rules.Chart, released []string) (string, error) {
		if len(released) == 0 {
			return "", nil
		}
		sorted, err := policy.SortNewestFirst(released, chart.PlainVersion)
		if err != nil {
			return "", fmt.Errorf("%s: %w", chart.Name, err)
		}
		return sorted[0], nil
	}

	main := members.Main()
	want, err := latest(main, mainReleased)
	if err != nil {
		return err
	}
	described := []string{describeLatest(main.Name, want)}

	var problems []error
	differ := false
	for _, chart := range members.Charts[1:] {
		released, err := published(ctx, cfg, chart, opts)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", chart.Name, err))
			continue
		}
		got, err := latest(chart, released)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		described = append(described, describeLatest(chart.Name, got))
		differ = differ || got != want
	}
	if differ {
		problems = append(problems, fmt.Errorf("the charts of the family %q are not at the same released version: %s: give --version to set it by hand",
			members.Name, strings.Join(described, ", ")))
	}
	return errors.Join(problems...)
}

func describeLatest(chart, latest string) string {
	if latest == "" {
		return chart + " (nothing released)"
	}
	return chart + " " + latest
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
