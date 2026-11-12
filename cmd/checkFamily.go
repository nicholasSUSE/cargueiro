package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/check"
	"github.com/rancher/cargueiro/internal/family"
	"github.com/rancher/cargueiro/internal/rules"
)

// loadFunc reads the chart at path: a directory or an archive.
type loadFunc func(path string) (*chart.Files, error)

// checkArgs is the number of arguments check takes: one chart, or with a
// family, one for each chart of it.
func checkArgs(name string) cobra.PositionalArgs {
	if name != "" {
		return cobra.MinimumNArgs(1)
	}
	return cobra.ExactArgs(1)
}

// checkFamily checks the charts at paths, which must be exactly the charts of the
// family called name, and prints a report for each of them in the order of the
// family: the main chart first. Every chart is checked, and the command fails if
// any is not valid, or if they do not all have the same version.
func checkFamily(cmd *cobra.Command, opts *rootOptions, name string, paths []string, load loadFunc, output string, noErrorHIP15 bool) error {
	if err := checkOutputFormat(output); err != nil {
		return err
	}
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	members, err := cfg.Family(name)
	if err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "checking family", "family", name, "charts", strings.Join(paths, ", "), "output", output)

	loaded, err := loadAll(paths, load)
	if err != nil {
		return err
	}
	ordered, err := family.Resolve(members, loaded, chartName)
	if err != nil {
		return err
	}

	results := make([]*check.Result, 0, len(ordered))
	for _, l := range ordered {
		result, err := check.Chart(cfg, l.files, l.files.Metadata.Version)
		if err != nil {
			return err
		}
		results = append(results, result)
	}
	if err := printResults(cmd.OutOrStdout(), results, output); err != nil {
		return fmt.Errorf("write result: %w", err)
	}

	var problems []error
	var invalid []string
	ignored := false
	for _, result := range results {
		switch {
		case result.Fails(noErrorHIP15):
			invalid = append(invalid, result.Chart)
		case !result.Valid:
			ignored = true
		}
	}
	if len(invalid) > 0 {
		problems = append(problems, fmt.Errorf("the family %q is not valid: %s", name, strings.Join(invalid, ", ")))
	}
	if err := sameVersion(name, results); err != nil {
		problems = append(problems, err)
	}
	if len(problems) > 0 {
		return errors.Join(problems...)
	}
	if ignored {
		_, err := fmt.Fprintln(cmd.ErrOrStderr(), "chart is not valid (HIP-15 errors ignored by --no-error-hip-15)")
		return err
	}
	return nil
}

// loadedChart is a chart that was read, and where from.
type loadedChart struct {
	path  string
	files *chart.Files
}

// chartName is the name of the chart in a folder or an archive that was read.
func chartName(l loadedChart) (string, error) {
	if l.files.Metadata.Name == "" {
		return "", fmt.Errorf("%s has no chart name", l.path)
	}
	return l.files.Metadata.Name, nil
}

// loadAll reads every chart, and reports every one that cannot be read.
func loadAll(paths []string, load loadFunc) ([]loadedChart, error) {
	loaded := make([]loadedChart, 0, len(paths))
	var problems []error
	for _, path := range paths {
		files, err := load(path)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		loaded = append(loaded, loadedChart{path: path, files: files})
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return loaded, nil
}

// sameVersion fails unless every chart of the family has the same version: a
// family is released at one version.
func sameVersion(name string, results []*check.Result) error {
	first := results[0].Metadata.Version
	same := true
	versions := make([]string, 0, len(results))
	for _, result := range results {
		versions = append(versions, result.Chart+" "+result.Metadata.Version)
		same = same && result.Metadata.Version == first
	}
	if same {
		return nil
	}
	return fmt.Errorf("the charts of the family %q do not have the same version: %s", name, strings.Join(versions, ", "))
}

// printResults prints one result for each chart: the text ones one after the
// other, with a blank line between, or a list in JSON.
func printResults(w io.Writer, results []*check.Result, output string) error {
	if output == outputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		// Version ranges hold < and >, which are not meant to be escaped.
		enc.SetEscapeHTML(false)
		return enc.Encode(results)
	}
	for i, result := range results {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, resultText(result)); err != nil {
			return err
		}
	}
	return nil
}
