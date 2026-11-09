package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/check"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
)

// The formats check chart can print.
const (
	outputText = "text"
	outputJSON = "json"
)

func newCheckChartCmd(opts *rootOptions) (*cobra.Command, error) {
	var output string
	var noErrorHIP15 bool

	cmd := &cobra.Command{
		Use:  "chart <chart-dir>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkChart(cmd, opts, args[0], output, noErrorHIP15)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", outputText, "output format: text or json")
	cmd.Flags().BoolVar(&noErrorHIP15, "no-error-hip-15", false, "do not fail because of helm.sh/images (HIP-15) errors")
	if err := applyDoc(cmd, "check-chart"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// checkChart checks the chart in dir and reports on it.
func checkChart(cmd *cobra.Command, opts *rootOptions, dir, output string, noErrorHIP15 bool) error {
	if err := checkOutputFormat(output); err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "checking chart", "chart", dir, "output", output)

	files, err := chart.Load(dir)
	if err != nil {
		return err
	}
	return report(cmd, opts, files, files.Metadata.Version, output, noErrorHIP15)
}

// checkOutputFormat rejects a format that cannot be printed.
func checkOutputFormat(output string) error {
	if output != outputText && output != outputJSON {
		return fmt.Errorf("unknown output format %q, want %s or %s", output, outputText, outputJSON)
	}
	return nil
}

// report checks files against the rules and prints the result. chartVersion is
// the version to validate. A chart that
// breaks the rules is printed and then reported as an error, so the exit code
// is 1. With noErrorHIP15, a chart whose only errors are HIP-15 ones is printed
// the same way, and the exit code is 0.
func report(cmd *cobra.Command, opts *rootOptions, files *chart.Files, chartVersion, output string, noErrorHIP15 bool) error {
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	result, err := check.Chart(cfg, files, chartVersion)
	if err != nil {
		return err
	}

	if err := printResult(cmd.OutOrStdout(), result, output); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	if result.Fails(noErrorHIP15) {
		return errors.New("chart is not valid")
	}
	if !result.Valid {
		_, err := fmt.Fprintln(cmd.ErrOrStderr(), "chart is not valid (HIP-15 errors ignored by --no-error-hip-15)")
		return err
	}
	return nil
}

func printResult(w io.Writer, result *check.Result, output string) error {
	if output == outputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		// Version ranges hold < and >, which are not meant to be escaped.
		enc.SetEscapeHTML(false)
		return enc.Encode(result)
	}
	_, err := io.WriteString(w, resultText(result))
	return err
}

// resultText is the result as plain text: the same fields as the JSON, with
// one line per error. An empty field prints without a value.
func resultText(result *check.Result) string {
	var b strings.Builder
	line := func(indent, key, value string) {
		b.WriteString(indent + key + ":")
		if value != "" {
			b.WriteString(" " + value)
		}
		b.WriteString("\n")
	}

	line("", "valid", fmt.Sprint(result.Valid))
	sectionText(&b, "annotations", result.Annotations)
	sectionText(&b, "hip15-annotations", result.HIP15Annotations)
	sectionText(&b, "version", result.Version)
	line("", "chart", result.Chart)
	line("", "kind", result.Kind)
	line("", "family", result.Family)
	line("", "metadata", "")
	line("  ", "name", result.Metadata.Name)
	line("  ", "version", result.Metadata.Version)
	line("  ", "appVersion", result.Metadata.AppVersion)
	line("  ", "icon", result.Metadata.Icon)
	line("  ", "description", result.Metadata.Description)
	line("  ", "kubeVersion", result.Metadata.KubeVersion)
	return b.String()
}

func sectionText(b *strings.Builder, name string, section check.Section) {
	status := "valid"
	if !section.Valid {
		status = "invalid"
	}
	b.WriteString(name + ": " + status + "\n")
	for _, problem := range section.Errors {
		b.WriteString("  - " + problem + "\n")
	}
}
