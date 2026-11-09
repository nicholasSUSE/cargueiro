// Package check validates a chart directory against the rules file.
package check

import (
	"github.com/nicholasSUSE/cargueiro/internal/annotations"
	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
	"github.com/nicholasSUSE/cargueiro/internal/version"
)

// Result is the outcome of checking one chart. The tags are the JSON the check
// command prints.
//
// The verdict comes first, then the chart: the order of the fields is the order
// of the JSON.
type Result struct {
	// Valid is true when every section is valid.
	Valid            bool    `json:"valid"`
	Annotations      Section `json:"annotations"`
	HIP15Annotations Section `json:"hip15-annotations"`
	Version          Section `json:"version"`

	Chart    string   `json:"chart"`
	Kind     string   `json:"kind"`
	Family   string   `json:"family"`
	Metadata Metadata `json:"metadata"`
}

// Metadata is the part of Chart.yaml that the result reports. KubeVersion is
// the catalog.cattle.io/kube-version annotation, the value the rules check, not
// Helm's own kubeVersion field.
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

// Chart checks files against the rules of the chart that Chart.yaml names.
// chartVersion is the version to validate: the one in Chart.yaml for a directory
// or an archive, the one in the reference for a chart in a registry. It fails
// when the rules file does not list the chart, because then nothing says which
// rules apply. Violations are not failures: they are in the Result.
func Chart(cfg *rules.Config, files *chart.Files, chartVersion string) (*Result, error) {
	rule, err := cfg.Find(files.Metadata.Name)
	if err != nil {
		return nil, err
	}

	versions := cfg.Versions()
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Chart:  rule.Name,
		Kind:   rule.Kind,
		Family: rule.Family,
		Metadata: Metadata{
			Name:        files.Metadata.Name,
			Version:     files.Metadata.Version,
			AppVersion:  files.Metadata.AppVersion,
			Icon:        files.Metadata.Icon,
			Description: files.Metadata.Description,
			KubeVersion: annotations.KubeVersionAnnotation(files.Annotations),
		},
		Annotations:      section(annotations.Validate(rule, files.Annotations)),
		HIP15Annotations: section(annotations.ValidateHelmImages(files.Annotations, files.Values)),
		Version:          section(versionProblems(policy, rule, chartVersion, files.Annotations)),
	}
	result.Valid = result.Annotations.Valid && result.HIP15Annotations.Valid && result.Version.Valid
	return result, nil
}

// Fails reports whether the command that printed the result must fail. HIP-15
// errors are left out when ignoreHIP15 is set; they stay in the result.
func (r *Result) Fails(ignoreHIP15 bool) bool {
	if ignoreHIP15 {
		return !r.Annotations.Valid || !r.Version.Valid
	}
	return !r.Valid
}

// versionProblems validates the form of chartVersion and, for a chart that
// carries a rancher-version annotation with a lower bound, its major. A CRD
// chart has no such annotation, and a missing or malformed one is reported by
// the annotations, so the major is left unchecked in those cases.
func versionProblems(policy *version.Policy, rule rules.Chart, chartVersion string, annotationMap map[string]string) []string {
	if err := policy.Validate(chartVersion, rule.PlainVersion); err != nil {
		return []string{err.Error()}
	}

	if rule.Kind == rules.KindCRD {
		return nil
	}
	rancher := annotations.RancherVersionAnnotation(annotationMap)
	if _, ok := version.RancherOf(rancher); !ok {
		return nil
	}
	if err := policy.CheckMajor(chartVersion, rancher); err != nil {
		return []string{err.Error()}
	}
	return nil
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
