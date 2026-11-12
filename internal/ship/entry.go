package ship

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"

	"github.com/rancher/cargueiro/internal/chart"
)

// BuildEntry returns the text of the entry of a chart version, from the text of
// its Chart.yaml: the same text, with the icon changed to icon when icon is not
// empty, and the date the chart was pushed and the URL the version is pulled
// from added at the end. Nothing else changes, comments and the order of the
// keys included.
//
// It fails when the result would not be an entry that cargueiro index reads: a
// Chart.yaml that has urls or created already, no date to write, or a field that
// the index does not know.
func BuildEntry(chartYAML []byte, icon, url string, created time.Time) ([]byte, error) {
	entry := bytes.Clone(chartYAML)
	if created.IsZero() {
		return nil, errors.New("the entry has no date for when the chart was pushed")
	}

	if icon != "" {
		var err error
		if entry, err = chart.SetIcon(entry, icon); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"created", "urls"} {
		if bytes.HasPrefix(entry, []byte(key+":")) || bytes.Contains(entry, []byte("\n"+key+":")) {
			return nil, fmt.Errorf("the Chart.yaml already has %s", key)
		}
	}

	newline := "\n"
	if bytes.Contains(entry, []byte("\r\n")) {
		newline = "\r\n"
	}
	if len(entry) > 0 && !bytes.HasSuffix(entry, []byte("\n")) {
		entry = append(entry, newline...)
	}
	entry = append(entry, "created: \""+created.UTC().Format(time.RFC3339)+"\""+newline...)
	entry = append(entry, "urls:"+newline+"- "+url+newline...)

	var parsed repo.ChartVersion
	if err := yaml.UnmarshalStrict(entry, &parsed); err != nil {
		return nil, fmt.Errorf("the entry would not be read by index: %w", err)
	}
	if parsed.Metadata == nil || parsed.Name == "" {
		return nil, errors.New("the entry would not be read by index: it has no name")
	}
	return entry, nil
}
