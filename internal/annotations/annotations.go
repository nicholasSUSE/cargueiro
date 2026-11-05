// Package annotations checks the catalog annotations of a chart against the
// rules of its kind.
package annotations

import (
	"fmt"
	"strings"

	"github.com/nicholasSUSE/cargueiro/internal/rules"
)

// prefix starts every annotation named in the rules file.
const prefix = "catalog.cattle.io/"

// Validate checks the annotations of chart against the rules of its kind and
// returns what is wrong, or nil when nothing is.
func Validate(chart rules.Chart, annotations map[string]string) []string {
	switch chart.Kind {
	case rules.KindMain:
		return ValidateMainChart(chart, annotations)
	case rules.KindCRD:
		return ValidateCRD(chart, annotations)
	case rules.KindExtra:
		return ValidateExtraChart(chart, annotations)
	default:
		return []string{fmt.Sprintf("chart %s has unknown kind %q", chart.Name, chart.Kind)}
	}
}

// ValidateMainChart checks the annotations of a main chart.
func ValidateMainChart(chart rules.Chart, annotations map[string]string) []string {
	return append(missing(chart, annotations), releaseName(chart, annotations)...)
}

// ValidateCRD checks the annotations of a CRD chart.
func ValidateCRD(chart rules.Chart, annotations map[string]string) []string {
	return append(missing(chart, annotations), releaseName(chart, annotations)...)
}

// ValidateExtraChart checks the annotations of an extra chart.
func ValidateExtraChart(chart rules.Chart, annotations map[string]string) []string {
	return missing(chart, annotations)
}

// missing reports each annotation the chart requires and lacks, in the order
// the rules file lists them. A blank value counts as lacking.
func missing(chart rules.Chart, annotations map[string]string) []string {
	var problems []string
	for _, name := range chart.Required {
		if strings.TrimSpace(annotations[prefix+name]) == "" {
			problems = append(problems, fmt.Sprintf("missing annotation %s%s", prefix, name))
		}
	}
	return problems
}

// releaseName reports a release-name annotation that differs from the one the
// rules file expects. An absent or blank one is left to missing.
func releaseName(chart rules.Chart, annotations map[string]string) []string {
	got := annotations[prefix+"release-name"]
	if strings.TrimSpace(got) == "" || got == chart.ReleaseName {
		return nil
	}
	return []string{fmt.Sprintf("%srelease-name is %q, want %q", prefix, got, chart.ReleaseName)}
}
