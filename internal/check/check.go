// Package check validates a chart directory against the rules file.
package check

import (
	"github.com/nicholasSUSE/cargueiro/internal/annotations"
	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
)

// Result is the outcome of checking one chart. The tags are the JSON the check
// command prints.
type Result struct {
	Chart  string `json:"chart"`
	Kind   string `json:"kind"`
	Family string `json:"family"`
	// Valid is true when every section is valid.
	Valid            bool     `json:"valid"`
	Metadata         Metadata `json:"metadata"`
	Annotations      Section  `json:"annotations"`
	HIP15Annotations Section  `json:"hip15-annotations"`
}

// Metadata is the part of Chart.yaml that the result reports.
type Metadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	AppVersion  string `json:"appVersion"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	KubeVersion string `json:"kubeVersion"`
}

// Section is the outcome of one group of checks. Errors is never nil, so it
// prints as [] and not as null.
type Section struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

// Chart checks files against the rules of the chart that Chart.yaml names. It
// fails when the rules file does not list that chart, because then nothing says
// which rules apply. Violations are not failures: they are in the Result.
func Chart(cfg *rules.Config, files *chart.Files) (*Result, error) {
	rule, err := cfg.Find(files.Metadata.Name)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Chart:            rule.Name,
		Kind:             rule.Kind,
		Family:           rule.Family,
		Metadata:         Metadata(files.Metadata),
		Annotations:      section(annotations.Validate(rule, files.Annotations)),
		HIP15Annotations: section(annotations.ValidateHelmImages(files.Annotations, files.Values)),
	}
	result.Valid = result.Annotations.Valid && result.HIP15Annotations.Valid
	return result, nil
}

func section(problems []string) Section {
	if problems == nil {
		problems = []string{}
	}
	return Section{
		Valid:  len(problems) == 0,
		Errors: problems,
	}
}
