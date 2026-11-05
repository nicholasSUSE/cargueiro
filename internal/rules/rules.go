// Package rules reads the rules file: which charts exist, what
// kind each one is, and which annotations each kind requires.
package rules

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// Chart is one chart listed in the rules file, with the rules of its kind.
type Chart struct {
	Name   string
	Kind   string
	Family string
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

// Config is the parsed rules file.
type Config struct {
	charts map[string]Chart
}

// Lookup returns the chart called name, and whether the file lists it.
func (c *Config) Lookup(name string) (Chart, bool) {
	chart, ok := c.charts[name]
	return chart, ok
}

type file struct {
	Kinds    map[string]kindRule `yaml:"kinds"`
	Families []family            `yaml:"families"`
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
}

// Parse reads a rules file. It rejects unknown keys, a chart listed twice,
// and a chart whose kind is not defined.
func Parse(data []byte) (*Config, error) {
	var f file
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("parse rules: empty file")
		}
		return nil, fmt.Errorf("parse rules: %w", err)
	}

	for kind, rule := range f.Kinds {
		if kind != KindMain && kind != KindCRD && kind != KindExtra {
			return nil, fmt.Errorf("parse rules: unknown kind %q", kind)
		}
		if err := checkRule(rule); err != nil {
			return nil, fmt.Errorf("parse rules: kind %q: %w", kind, err)
		}
	}

	charts := make(map[string]Chart)
	for _, fam := range f.Families {
		if fam.Family == "" {
			return nil, errors.New("parse rules: a family has no name")
		}
		if len(fam.Charts) == 0 {
			return nil, fmt.Errorf("parse rules: family %q has no charts", fam.Family)
		}

		for _, item := range fam.Charts {
			if item.Name == "" {
				return nil, fmt.Errorf("parse rules: family %q has a chart with no name", fam.Family)
			}
			if _, dup := charts[item.Name]; dup {
				return nil, fmt.Errorf("parse rules: chart %q is listed twice", item.Name)
			}
			rule, ok := f.Kinds[item.Kind]
			if !ok {
				return nil, fmt.Errorf("parse rules: chart %q has undefined kind %q", item.Name, item.Kind)
			}

			releaseName := item.ReleaseName
			if releaseName == "" {
				releaseName = item.Name
			}
			charts[item.Name] = Chart{
				Name:        item.Name,
				Kind:        item.Kind,
				Family:      fam.Family,
				ReleaseName: releaseName,
				Required:    rule.Required,
			}
		}
	}
	if len(charts) == 0 {
		return nil, errors.New("parse rules: no charts listed")
	}

	return &Config{charts: charts}, nil
}

func checkRule(rule kindRule) error {
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
