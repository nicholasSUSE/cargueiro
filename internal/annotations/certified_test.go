package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckCertified(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "rancher", value: "rancher"},

		{name: "absent is left to checkMissing", value: ""},
		{name: "blank is left to checkMissing", value: "  "},

		{name: "another value", value: "partner", want: []string{`catalog.cattle.io/certified "partner": must be "rancher"`}},
		{name: "different case", value: "Rancher", want: []string{`catalog.cattle.io/certified "Rancher": must be "rancher"`}},
		{name: "surrounding space is ignored", value: "  rancher "},
		{name: "boolean", value: "true", want: []string{`catalog.cattle.io/certified "true": must be "rancher"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkCertified(map[string]string{"catalog.cattle.io/certified": tt.value})
			require.Equal(t, tt.want, got)
		})
	}

	require.Empty(t, checkCertified(nil))
	require.Empty(t, checkCertified(map[string]string{"catalog.rancher.io/certified": "partner"}))
}
