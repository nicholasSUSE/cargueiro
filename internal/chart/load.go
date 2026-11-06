package chart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// Metadata is the part of Chart.yaml that check reports.
type Metadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"appVersion"`
	Icon        string `yaml:"icon"`
	Description string `yaml:"description"`
}

// Files holds Chart.yaml and values.yaml of a chart directory, read into memory.
type Files struct {
	Metadata    Metadata
	Annotations map[string]string
	Values      map[string]any
}

// chartYAML is the shape of Chart.yaml that Load reads. Fields it does not
// list are ignored.
type chartYAML struct {
	Metadata    `yaml:",inline"`
	Annotations map[string]string `yaml:"annotations"`
}

// Load reads Chart.yaml and values.yaml from dir. Both files must exist and be
// valid YAML. Nothing else in dir is read.
func Load(dir string) (*Files, error) {
	var meta chartYAML
	if err := readYAML(filepath.Join(dir, "Chart.yaml"), &meta); err != nil {
		return nil, err
	}

	var values map[string]any
	if err := readYAML(filepath.Join(dir, "values.yaml"), &values); err != nil {
		return nil, err
	}

	return &Files{
		Metadata:    meta.Metadata,
		Annotations: meta.Annotations,
		Values:      values,
	}, nil
}

// readYAML decodes the YAML file at path into out. An empty file leaves out
// untouched.
func readYAML(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found", path)
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
