package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckHidden(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "true", value: "true"},

		{name: "absent is left to checkMissing", value: ""},
		{name: "blank is left to checkMissing", value: "  "},

		{name: "false", value: "false", want: []string{`catalog.cattle.io/hidden "false": must be "true"`}},
		{name: "yes", value: "yes", want: []string{`catalog.cattle.io/hidden "yes": must be "true"`}},
		{name: "different case", value: "True", want: []string{`catalog.cattle.io/hidden "True": must be "true"`}},
		{name: "surrounding space is ignored", value: " true  "},
		{name: "number", value: "1", want: []string{`catalog.cattle.io/hidden "1": must be "true"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkHidden(map[string]string{"catalog.cattle.io/hidden": tt.value})
			require.Equal(t, tt.want, got)
		})
	}

	require.Empty(t, checkHidden(nil))
	require.Empty(t, checkHidden(map[string]string{"catalog.rancher.io/hidden": "yes"}))
}
