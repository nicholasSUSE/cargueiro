package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/check"
	"github.com/rancher/cargueiro/internal/family"
	"github.com/rancher/cargueiro/internal/rules"
)

// packageFamily packages the charts of the family called name: dirs must be the
// folders of exactly its charts, in any order. Every chart is checked before any
// archive is built. It returns the archives, in the order of the family, the main
// chart first. When a chart fails, the archives already built are removed.
func packageFamily(cmd *cobra.Command, opts *rootOptions, pkg packageFunc, name string, dirs []string, outputDir string) ([]string, error) {
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return nil, err
	}
	members, err := cfg.Family(name)
	if err != nil {
		return nil, err
	}
	loaded, err := loadAll(dirs, chart.Load)
	if err != nil {
		return nil, err
	}
	ordered, err := family.Resolve(members, loaded, chartName)
	if err != nil {
		return nil, err
	}
	opts.log.DebugContext(cmd.Context(), "packaging family", "family", name, "output", outputDir)

	if err := requireValidFamily(cmd, cfg, name, ordered); err != nil {
		return nil, err
	}

	_, statErr := os.Stat(outputDir)
	existed := statErr == nil
	archives := make([]string, 0, len(ordered))
	for _, l := range ordered {
		archive, err := packageChart(cmd, opts, pkg, l.path, outputDir)
		if err != nil {
			err = fmt.Errorf("%s: %w", l.files.Metadata.Name, err)
			return nil, errors.Join(err, removeArchives(archives, outputDir, existed))
		}
		archives = append(archives, archive)
	}
	return archives, nil
}

// requireValidFamily runs the checks of check chart on every chart of the
// family, writes the report of each one that is not valid to stderr, and fails
// when any is not valid or when the charts do not all have the same version.
func requireValidFamily(cmd *cobra.Command, cfg *rules.Config, name string, ordered []loadedChart) error {
	results := make([]*check.Result, 0, len(ordered))
	var problems []error
	for _, l := range ordered {
		result, err := check.Chart(cfg, l.files, l.files.Metadata.Version)
		if err != nil {
			return err
		}
		results = append(results, result)
		if result.Valid {
			continue
		}
		if _, err := fmt.Fprint(cmd.ErrOrStderr(), resultText(result)); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
		problems = append(problems, fmt.Errorf("%s: the chart is not valid, so the family was not packaged", result.Chart))
	}
	if err := sameVersion(name, results); err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

// removeArchives removes the archives that were built, and the output folder
// when it was made for them and is empty again.
func removeArchives(archives []string, outputDir string, existed bool) error {
	var problems []error
	for _, archive := range archives {
		if err := os.Remove(archive); err != nil && !errors.Is(err, os.ErrNotExist) {
			problems = append(problems, fmt.Errorf("remove %s: %w", archive, err))
		}
	}
	if !existed {
		_ = os.Remove(outputDir) // it stays if something else is in it
	}
	return errors.Join(problems...)
}
