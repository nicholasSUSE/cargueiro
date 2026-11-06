// Package annotations checks the catalog annotations of a chart against the
// rules of its kind. The helm.sh/images checks follow Helm HIP-15 and add
// Rancher policy where the comments say so.
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
	return collect(
		checkMissing(chart, annotations),
		checkReleaseName(chart, annotations),
		checkCertified(annotations),
		checkKubeVersion(annotations),
		checkRancherVersion(annotations),
		checkPermitsOS(annotations),
	)
}

// ValidateCRD checks the annotations of a CRD chart.
func ValidateCRD(chart rules.Chart, annotations map[string]string) []string {
	return collect(
		checkMissing(chart, annotations),
		checkReleaseName(chart, annotations),
		checkCertified(annotations),
		checkHidden(annotations),
	)
}

// ValidateExtraChart checks the annotations of an extra chart.
func ValidateExtraChart(chart rules.Chart, annotations map[string]string) []string {
	return collect(
		checkMissing(chart, annotations),
		checkCertified(annotations),
		checkKubeVersion(annotations),
		checkRancherVersion(annotations),
		checkPermitsOS(annotations),
	)
}

// collect joins the problems found by each check, in order.
func collect(checks ...[]string) []string {
	var problems []string
	for _, found := range checks {
		problems = append(problems, found...)
	}
	return problems
}

// annotationValue returns the annotation called name, without surrounding
// space. Every check reads annotations through it, so space around a value
// never matters.
func annotationValue(annotations map[string]string, name string) string {
	return strings.TrimSpace(annotations[prefix+name])
}

// checkValue reads the annotation called name and returns the problem validate
// finds in its value. An absent or blank annotation is left to checkMissing.
func checkValue(annotations map[string]string, name string, validate func(value string) error) []string {
	value := annotationValue(annotations, name)
	if value == "" {
		return nil
	}
	if err := validate(value); err != nil {
		return []string{fmt.Sprintf("%s%s %q: %v", prefix, name, value, err)}
	}
	return nil
}

// checkMissing reports each annotation the chart requires and lacks, in the order
// the rules file lists them. A blank value counts as lacking.
func checkMissing(chart rules.Chart, annotations map[string]string) []string {
	var problems []string
	for _, name := range chart.Required {
		if annotationValue(annotations, name) == "" {
			problems = append(problems, fmt.Sprintf("missing annotation %s%s", prefix, name))
		}
	}
	return problems
}

// checkReleaseName reports a release-name annotation that differs from the one the
// rules file expects. An absent or blank one is left to checkMissing.
func checkReleaseName(chart rules.Chart, annotations map[string]string) []string {
	got := annotationValue(annotations, "release-name")
	if got == "" || got == chart.ReleaseName {
		return nil
	}
	return []string{fmt.Sprintf("%srelease-name is %q, want %q", prefix, got, chart.ReleaseName)}
}
