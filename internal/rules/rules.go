// Package rules reads the rules file: which charts exist, what
// kind each one is, and which annotations each kind requires.
package rules

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Chart is one chart listed in the rules file, with the rules of its kind.
type Chart struct {
	Name   string
	Kind   string
	Family string
	// Edition is the repository the chart is released from.
	Edition string
	// PlainVersion is true when the chart version has no +up part: 1XY.Z.W
	// and not 1XY.Z.W+upA.B.C.
	PlainVersion bool
	// ReleaseName is the expected catalog.cattle.io/release-name. It is the
	// chart name unless the file says otherwise.
	ReleaseName string
	// Required lists the annotations the chart must carry, without the
	// catalog.cattle.io/ prefix.
	Required []string
}

// The kinds a rules file may define.
const (
	KindMain  = "main"
	KindCRD   = "crd"
	KindExtra = "extra"
)

// versionPlain is the value of a chart's version key for a chart whose version
// has no +up part.
const versionPlain = "plain"

// The editions a chart may belong to.
const (
	EditionRancherCharts = "rancher-charts"
	EditionPrimeCharts   = "prime-charts"
)

// ChartRegistries are the repositories an edition releases its charts to, as
// host/path with no scheme and no trailing slash.
type ChartRegistries struct {
	Staging    string
	Production string
}

// ImageRegistries are the repositories images are released to. Both editions
// share them.
type ImageRegistries struct {
	Staging    string
	Production string
}

// Registries are the registries named in the rules file.
type Registries struct {
	RancherCharts ChartRegistries
	PrimeCharts   ChartRegistries
	Images        ImageRegistries
}

// Versions is the chart version policy named in the rules file.
type Versions struct {
	// Prereleases are the prerelease names a chart version may carry, as in
	// the rc of -rc.1.
	Prereleases []string
	// Majors maps a Rancher version, such as 2.16, to the major its charts
	// carry, such as 111.
	Majors map[string]int
}

// ChartRepositories lists the repositories the charts of edition are released
// to, staging first. It is empty for an unknown edition.
func (r Registries) ChartRepositories(edition string) []string {
	switch edition {
	case EditionRancherCharts:
		return []string{r.RancherCharts.Staging, r.RancherCharts.Production}
	case EditionPrimeCharts:
		return []string{r.PrimeCharts.Staging, r.PrimeCharts.Production}
	}
	return nil
}

// ImageRepositories lists the repositories images are released to, staging
// first.
func (r Registries) ImageRepositories() []string {
	return []string{r.Images.Staging, r.Images.Production}
}

// Config is the parsed rules file.
type Config struct {
	charts     map[string]Chart
	registries Registries
	versions   Versions
}

// Versions returns the chart version policy named in the rules file.
func (c *Config) Versions() Versions {
	return Versions{
		Prereleases: slices.Clone(c.versions.Prereleases),
		Majors:      maps.Clone(c.versions.Majors),
	}
}

// Editions returns the edition of every listed chart, by chart name.
func (c *Config) Editions() map[string]string {
	editions := make(map[string]string, len(c.charts))
	for name, chart := range c.charts {
		editions[name] = chart.Edition
	}
	return editions
}

// Registries returns the registries named in the rules file.
func (c *Config) Registries() Registries {
	return c.registries
}

// Find returns the chart called name. It fails when the rules file does not
// list the chart, because then nothing says which rules apply to it.
func (c *Config) Find(name string) (Chart, error) {
	chart, ok := c.charts[name]
	if !ok {
		return Chart{}, fmt.Errorf("chart %q is not listed in the rules file (rules.yaml)", name)
	}
	return chart, nil
}

type file struct {
	Kinds    map[string]kindRule `yaml:"kinds"`
	Families []family            `yaml:"families"`

	Registries registriesFile `yaml:"registries"`
	Versions   versionsFile   `yaml:"versions"`
}

type versionsFile struct {
	Prereleases []string       `yaml:"prereleases"`
	Majors      map[string]int `yaml:"majors"`
}

type registriesFile struct {
	RancherCharts chartRegistriesFile `yaml:"rancher-charts"`
	PrimeCharts   chartRegistriesFile `yaml:"prime-charts"`
	Images        imageRegistriesFile `yaml:"images"`
}

type chartRegistriesFile struct {
	Staging    string `yaml:"charts-staging"`
	Production string `yaml:"charts-production"`
}

type imageRegistriesFile struct {
	Staging    string `yaml:"staging"`
	Production string `yaml:"production"`
}

type kindRule struct {
	Required []string `yaml:"required"`
}

type family struct {
	Family string      `yaml:"family"`
	Charts []chartItem `yaml:"charts"`
}

type chartItem struct {
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"`
	ReleaseName string `yaml:"release-name"`
	Edition     string `yaml:"edition"`
	Version     string `yaml:"version"`
}

// Parse reads a rules file. It rejects malformed input: unknown keys, an
// unknown kind, a family or chart without a name, a family without charts, a
// chart listed twice or without a defined kind, a chart without a known
// edition, a family that mixes editions, and an empty or repeated annotation
// name in a kind, and a registry that is missing, has a scheme or ends in a
// slash.
func Parse(data []byte) (*Config, error) {
	f, err := decode(data)
	if err != nil {
		return nil, err
	}
	if err := checkKinds(f.Kinds); err != nil {
		return nil, err
	}

	charts, err := indexCharts(f)
	if err != nil {
		return nil, err
	}
	registries, err := newRegistries(f.Registries)
	if err != nil {
		return nil, err
	}
	versions, err := newVersions(f.Versions)
	if err != nil {
		return nil, err
	}
	return &Config{charts: charts, registries: registries, versions: versions}, nil
}

// decode reads the YAML, rejecting unknown keys.
func decode(data []byte) (file, error) {
	var f file
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return file{}, errors.New("parse rules: empty file")
		}
		return file{}, fmt.Errorf("parse rules: %w", err)
	}
	return f, nil
}

// checkKinds rejects a kind that has no validation function and a bad list of
// required annotations. The kinds are taken in order, so the same file always
// reports the same error.
func checkKinds(kinds map[string]kindRule) error {
	for _, kind := range slices.Sorted(maps.Keys(kinds)) {
		if kind != KindMain && kind != KindCRD && kind != KindExtra {
			return fmt.Errorf("parse rules: unknown kind %q", kind)
		}
		if err := checkRequired(kinds[kind]); err != nil {
			return fmt.Errorf("parse rules: kind %q: %w", kind, err)
		}
	}
	return nil
}

// indexCharts lists every chart of every family by name.
func indexCharts(f file) (map[string]Chart, error) {
	charts := make(map[string]Chart)
	for _, fam := range f.Families {
		if fam.Family == "" {
			return nil, errors.New("parse rules: a family has no name")
		}
		if len(fam.Charts) == 0 {
			return nil, fmt.Errorf("parse rules: family %q has no charts", fam.Family)
		}

		for _, item := range fam.Charts {
			chart, err := newChart(fam.Family, item, f.Kinds)
			if err != nil {
				return nil, err
			}
			if chart.Edition != fam.Charts[0].Edition {
				return nil, fmt.Errorf("parse rules: family %q mixes editions %q and %q",
					fam.Family, fam.Charts[0].Edition, chart.Edition)
			}
			if _, dup := charts[chart.Name]; dup {
				return nil, fmt.Errorf("parse rules: chart %q is listed twice", chart.Name)
			}
			charts[chart.Name] = chart
		}
	}
	if len(charts) == 0 {
		return nil, errors.New("parse rules: no charts listed")
	}
	return charts, nil
}

// newChart builds the chart described by item, with the rules of its kind.
func newChart(family string, item chartItem, kinds map[string]kindRule) (Chart, error) {
	if item.Name == "" {
		return Chart{}, fmt.Errorf("parse rules: family %q has a chart with no name", family)
	}

	rule, defined := kinds[item.Kind]
	switch {
	case item.Kind == "":
		return Chart{}, fmt.Errorf("parse rules: chart %q has no kind", item.Name)
	case !defined:
		return Chart{}, fmt.Errorf("parse rules: chart %q has undefined kind %q", item.Name, item.Kind)
	}

	switch item.Edition {
	case "":
		return Chart{}, fmt.Errorf("parse rules: chart %q has no edition", item.Name)
	case EditionRancherCharts, EditionPrimeCharts:
	default:
		return Chart{}, fmt.Errorf("parse rules: chart %q has unknown edition %q", item.Name, item.Edition)
	}

	switch item.Version {
	case "", versionPlain:
	default:
		return Chart{}, fmt.Errorf("parse rules: chart %q has unknown version %q", item.Name, item.Version)
	}

	releaseName := item.ReleaseName
	if releaseName == "" {
		releaseName = item.Name
	}
	return Chart{
		Name:         item.Name,
		Kind:         item.Kind,
		Family:       family,
		Edition:      item.Edition,
		ReleaseName:  releaseName,
		PlainVersion: item.Version == versionPlain,
		Required:     slices.Clone(rule.Required),
	}, nil
}

// checkRequired rejects an empty or repeated annotation name in a kind.
func checkRequired(rule kindRule) error {
	seen := make(map[string]bool, len(rule.Required))
	for _, name := range rule.Required {
		if name == "" {
			return errors.New("empty annotation name")
		}
		if seen[name] {
			return fmt.Errorf("annotation %q is required twice", name)
		}
		seen[name] = true
	}
	return nil
}

// newRegistries checks every registry and returns them as typed fields.
func newRegistries(r registriesFile) (Registries, error) {
	fields := []struct {
		key   string
		value string
	}{
		{key: "rancher-charts.charts-staging", value: r.RancherCharts.Staging},
		{key: "rancher-charts.charts-production", value: r.RancherCharts.Production},
		{key: "prime-charts.charts-staging", value: r.PrimeCharts.Staging},
		{key: "prime-charts.charts-production", value: r.PrimeCharts.Production},
		{key: "images.staging", value: r.Images.Staging},
		{key: "images.production", value: r.Images.Production},
	}
	for _, field := range fields {
		if err := checkRegistry(field.value); err != nil {
			return Registries{}, fmt.Errorf("parse rules: registry %q: %w", field.key, err)
		}
	}

	return Registries{
		RancherCharts: ChartRegistries(r.RancherCharts),
		PrimeCharts:   ChartRegistries(r.PrimeCharts),
		Images:        ImageRegistries(r.Images),
	}, nil
}

// checkRegistry rejects a registry that is empty, has a scheme or ends in a
// slash.
func checkRegistry(value string) error {
	switch {
	case value == "":
		return errors.New("missing")
	case strings.Contains(value, "://"):
		return errors.New("must not have a scheme")
	case strings.HasSuffix(value, "/"):
		return errors.New("must not end in a slash")
	}
	return nil
}

var (
	prereleaseName = regexp.MustCompile(`^[a-z]+$`)
	rancherVersion = regexp.MustCompile(`^2\.\d+$`)
)

// The majors a chart version may start with: 1XY.
const (
	minMajor = 100
	maxMajor = 199
)

// newVersions checks the version policy and returns it.
func newVersions(v versionsFile) (Versions, error) {
	if len(v.Prereleases) == 0 {
		return Versions{}, errors.New("parse rules: versions: no prereleases")
	}
	seen := make(map[string]bool, len(v.Prereleases))
	for _, name := range v.Prereleases {
		switch {
		case !prereleaseName.MatchString(name):
			return Versions{}, fmt.Errorf("parse rules: versions: prerelease %q must be lowercase letters", name)
		case seen[name]:
			return Versions{}, fmt.Errorf("parse rules: versions: prerelease %q is listed twice", name)
		}
		seen[name] = true
	}

	if len(v.Majors) == 0 {
		return Versions{}, errors.New("parse rules: versions: no majors")
	}
	used := make(map[int]string, len(v.Majors))
	for _, rancher := range slices.Sorted(maps.Keys(v.Majors)) {
		major := v.Majors[rancher]
		switch {
		case !rancherVersion.MatchString(rancher):
			return Versions{}, fmt.Errorf("parse rules: versions: Rancher version %q must look like 2.16", rancher)
		case major < minMajor || major > maxMajor:
			return Versions{}, fmt.Errorf("parse rules: versions: major %d of Rancher %s must be between %d and %d",
				major, rancher, minMajor, maxMajor)
		case used[major] != "":
			return Versions{}, fmt.Errorf("parse rules: versions: major %d is used by Rancher %s and %s", major, used[major], rancher)
		}
		used[major] = rancher
	}

	return Versions{
		Prereleases: slices.Clone(v.Prereleases),
		Majors:      maps.Clone(v.Majors),
	}, nil
}
