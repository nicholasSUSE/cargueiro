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

	cmd := &cobra.Command{
		Use:  "chart <chart-dir>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkChart(cmd, opts, args[0], output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", outputText, "output format: text or json")
	if err := applyDoc(cmd, "check-chart"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// checkChart checks the chart in dir and prints the result. A chart that breaks
// the rules is printed and then reported as an error, so the exit code is 1.
func checkChart(cmd *cobra.Command, opts *rootOptions, dir, output string) error {
	if output != outputText && output != outputJSON {
		return fmt.Errorf("unknown output format %q, want %s or %s", output, outputText, outputJSON)
	}
	opts.log.DebugContext(cmd.Context(), "checking chart", "chart", dir, "output", output)

	files, err := chart.Load(dir)
	if err != nil {
		return err
	}
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	result, err := check.Chart(cfg, files)
	if err != nil {
		return err
	}

	if err := printResult(cmd.OutOrStdout(), result, output); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	if !result.Valid {
		return errors.New("chart is not valid")
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

	line("", "chart", result.Chart)
	line("", "kind", result.Kind)
	line("", "family", result.Family)
	line("", "valid", fmt.Sprint(result.Valid))
	line("", "metadata", "")
	line("  ", "name", result.Metadata.Name)
	line("  ", "version", result.Metadata.Version)
	line("  ", "appVersion", result.Metadata.AppVersion)
	line("  ", "icon", result.Metadata.Icon)
	line("  ", "description", result.Metadata.Description)
	line("  ", "kubeVersion", result.Metadata.KubeVersion)
	sectionText(&b, "annotations", result.Annotations)
	sectionText(&b, "hip15-annotations", result.HIP15Annotations)
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
