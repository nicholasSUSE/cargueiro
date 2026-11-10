package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	helmchart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"

	"github.com/rancher/cargueiro/internal/chart"
)

// checkArchive fails unless archive is the chart the entry describes: the
// Chart.yaml inside it has the same metadata as the entry, whose icon is the
// only part that is not compared, because the entry points it to the logo kept
// in the charts repository.
func checkArchive(entry *repo.ChartVersion, archive []byte) error {
	data, err := chart.ArchiveChartYAML(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("%s %s: read the archive: %w", entry.Name, entry.Version, err)
	}
	var inside helmchart.Metadata
	if err := yaml.Unmarshal(data, &inside); err != nil {
		return fmt.Errorf("%s %s: read the Chart.yaml in the archive: %w", entry.Name, entry.Version, err)
	}

	differing, err := differingFields(entry.Metadata, &inside)
	if err != nil {
		return fmt.Errorf("%s %s: %w", entry.Name, entry.Version, err)
	}
	if len(differing) > 0 {
		return fmt.Errorf("%s %s: the Chart.yaml in the archive differs from the entry in: %s", entry.Name, entry.Version, strings.Join(differing, ", "))
	}
	return nil
}

// differingFields returns the fields, by their name in Chart.yaml, in which a
// and b differ, sorted. The icon is left out.
func differingFields(a, b *helmchart.Metadata) ([]string, error) {
	x, err := fields(a)
	if err != nil {
		return nil, err
	}
	y, err := fields(b)
	if err != nil {
		return nil, err
	}

	var differing []string
	for name := range maps.Keys(x) {
		if !reflect.DeepEqual(x[name], y[name]) {
			differing = append(differing, name)
		}
	}
	for name := range maps.Keys(y) {
		if _, found := x[name]; !found {
			differing = append(differing, name)
		}
	}
	slices.Sort(differing)
	return slices.Compact(differing), nil
}

// fields returns the metadata as its fields in Chart.yaml, with the icon left out.
func fields(m *helmchart.Metadata) (map[string]any, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("compare the metadata: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("compare the metadata: %w", err)
	}
	delete(out, "icon")
	return out, nil
}
