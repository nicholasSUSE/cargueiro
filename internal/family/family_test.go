package family

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/rules"
)

func fleet() rules.Family {
	return rules.Family{
		Name:    "fleet",
		Edition: rules.EditionRancherCharts,
		Charts: []rules.Chart{
			{Name: "fleet", Kind: rules.KindMain},
			{Name: "fleet-agent", Kind: rules.KindExtra},
			{Name: "fleet-crd", Kind: rules.KindCRD},
		},
	}
}

// The items are folders, and a folder is named after the chart it holds.
func nameOf(item string) (string, error) {
	if item == "unreadable" {
		return "", errors.New("read unreadable/Chart.yaml: no such file")
	}
	return item, nil
}

func TestResolveOrdersByTheFamily(t *testing.T) {
	tests := []struct {
		name  string
		given []string
		want  []string
	}{
		{
			name:  "already in order",
			given: []string{"fleet", "fleet-agent", "fleet-crd"},
			want:  []string{"fleet", "fleet-agent", "fleet-crd"},
		},
		{
			name:  "the main chart is put first",
			given: []string{"fleet-crd", "fleet-agent", "fleet"},
			want:  []string{"fleet", "fleet-agent", "fleet-crd"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(fleet(), tt.given, nameOf)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResolveRejects(t *testing.T) {
	tests := []struct {
		name    string
		given   []string
		wantErr []string
	}{
		{
			name:    "a chart is missing",
			given:   []string{"fleet", "fleet-crd"},
			wantErr: []string{`the family "fleet" is missing fleet-agent`},
		},
		{
			name:    "several charts are missing",
			given:   []string{"fleet"},
			wantErr: []string{`the family "fleet" is missing fleet-agent, fleet-crd`},
		},
		{
			name:    "nothing was given",
			given:   nil,
			wantErr: []string{`the family "fleet" is missing fleet, fleet-agent, fleet-crd`},
		},
		{
			name:    "a chart of another family",
			given:   []string{"fleet", "fleet-agent", "fleet-crd", "longhorn"},
			wantErr: []string{`chart "longhorn" is not in the family "fleet"`},
		},
		{
			name:    "a chart is given twice",
			given:   []string{"fleet", "fleet", "fleet-agent", "fleet-crd"},
			wantErr: []string{`chart "fleet" is given twice`},
		},
		{
			name:  "every problem is reported",
			given: []string{"fleet", "fleet", "longhorn"},
			wantErr: []string{
				`chart "fleet" is given twice`,
				`chart "longhorn" is not in the family "fleet"`,
				`the family "fleet" is missing fleet-agent, fleet-crd`,
			},
		},
		{
			name:    "a chart that cannot be named is not reported as missing",
			given:   []string{"fleet", "unreadable", "fleet-crd"},
			wantErr: []string{"read unreadable/Chart.yaml: no such file"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(fleet(), tt.given, nameOf)

			require.Error(t, err)
			require.Nil(t, got)
			for _, want := range tt.wantErr {
				require.ErrorContains(t, err, want)
			}
		})
	}
}

func TestResolveDoesNotReportAMissingChartWhenOneCannotBeNamed(t *testing.T) {
	_, err := Resolve(fleet(), []string{"fleet", "unreadable"}, nameOf)

	require.Error(t, err)
	require.NotContains(t, err.Error(), "missing")
}

func TestResolveAFamilyOfOne(t *testing.T) {
	alone := rules.Family{Name: "alone", Charts: []rules.Chart{{Name: "alone", Kind: rules.KindMain}}}

	got, err := Resolve(alone, []string{"alone"}, nameOf)

	require.NoError(t, err)
	require.Equal(t, []string{"alone"}, got)
}
