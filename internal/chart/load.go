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

// Load reads Chart.yaml and values.yaml from dir. Chart.yaml must exist. A chart
// without a values.yaml has no values and no images, which is the case of most
// CRD charts. Both files must be valid YAML. Nothing else in dir is read.
func Load(dir string) (*Files, error) {
	chartPath := filepath.Join(dir, "Chart.yaml")
	chartData, err := readFile(chartPath)
	if err != nil {
		return nil, err
	}
	valuesPath := filepath.Join(dir, "values.yaml")
	valuesData, err := readFile(valuesPath)
	if err != nil && !errors.Is(err, errNotFound) {
		return nil, err
	}

	return newFiles(chartPath, chartData, valuesPath, valuesData)
}

// errNotFound is wrapped by the error for a file that does not exist.
var errNotFound = errors.New("not found")

// readFile reads the file at path.
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s %w", path, errNotFound)
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// newFiles decodes the content of Chart.yaml and values.yaml. An empty file
// leaves its part of the result empty. The names are only used in errors.
func newFiles(chartName string, chartData []byte, valuesName string, valuesData []byte) (*Files, error) {
	var meta chartYAML
	if err := yaml.Unmarshal(chartData, &meta); err != nil {
		return nil, fmt.Errorf("parse %s: %w", chartName, err)
	}

	var values map[string]any
	if err := yaml.Unmarshal(valuesData, &values); err != nil {
		return nil, fmt.Errorf("parse %s: %w", valuesName, err)
	}

	return &Files{
		Metadata:    meta.Metadata,
		Annotations: meta.Annotations,
		Values:      values,
	}, nil
}
