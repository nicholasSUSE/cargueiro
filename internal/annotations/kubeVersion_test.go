package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckKubeVersion(t *testing.T) {
	const form = `must look like ">= 1.34.0-0 < 1.37.0-0"`

	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "valid", value: ">= 1.34.0-0 < 1.37.0-0"},
		{name: "valid without space after the operator", value: ">=1.18.0-0 < 1.37.0-0"},
		{name: "valid with surrounding space", value: "  >= 1.34.0-0 < 1.37.0-0 "},
		{name: "valid with a patch version", value: ">= 1.34.2-0 < 1.37.1-0"},

		{name: "absent is left to missing", value: ""},
		{name: "blank is left to missing", value: "  "},

		{name: "upper bound only", value: "< 1.37.0-0", want: []string{`catalog.cattle.io/kube-version "< 1.37.0-0": ` + form}},
		{name: "lower bound only", value: ">= 1.34.0-0", want: []string{`catalog.cattle.io/kube-version ">= 1.34.0-0": ` + form}},
		{name: "prerelease suffix missing", value: ">= 1.34.0 < 1.37.0", want: []string{`catalog.cattle.io/kube-version ">= 1.34.0 < 1.37.0": ` + form}},
		{name: "prerelease suffix on one side only", value: ">= 1.34.0-0 < 1.37.0", want: []string{`catalog.cattle.io/kube-version ">= 1.34.0-0 < 1.37.0": ` + form}},
		{name: "other prerelease", value: ">= 1.34.0-rc1 < 1.37.0-0", want: []string{`catalog.cattle.io/kube-version ">= 1.34.0-rc1 < 1.37.0-0": ` + form}},
		{name: "partial version", value: ">= 1.34-0 < 1.37.0-0", want: []string{`catalog.cattle.io/kube-version ">= 1.34-0 < 1.37.0-0": ` + form}},
		{name: "bounds reversed in text", value: "< 1.37.0-0 >= 1.34.0-0", want: []string{`catalog.cattle.io/kube-version "< 1.37.0-0 >= 1.34.0-0": ` + form}},
		{name: "inclusive upper bound", value: ">= 1.34.0-0 <= 1.37.0-0", want: []string{`catalog.cattle.io/kube-version ">= 1.34.0-0 <= 1.37.0-0": ` + form}},
		{name: "exclusive lower bound", value: "> 1.34.0-0 < 1.37.0-0", want: []string{`catalog.cattle.io/kube-version "> 1.34.0-0 < 1.37.0-0": ` + form}},
		{name: "union", value: ">= 1.30.0-0 < 1.32.0-0 || >= 1.34.0-0 < 1.37.0-0", want: []string{`catalog.cattle.io/kube-version ">= 1.30.0-0 < 1.32.0-0 || >= 1.34.0-0 < 1.37.0-0": ` + form}},
		{name: "text", value: "latest", want: []string{`catalog.cattle.io/kube-version "latest": ` + form}},

		{name: "lower above upper", value: ">= 1.37.0-0 < 1.34.0-0", want: []string{`catalog.cattle.io/kube-version ">= 1.37.0-0 < 1.34.0-0": lower bound 1.37.0-0 must be below upper bound 1.34.0-0`}},
		{name: "lower equals upper", value: ">= 1.34.0-0 < 1.34.0-0", want: []string{`catalog.cattle.io/kube-version ">= 1.34.0-0 < 1.34.0-0": lower bound 1.34.0-0 must be below upper bound 1.34.0-0`}},
		{name: "numeric comparison, not text", value: ">= 1.9.0-0 < 1.10.0-0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkKubeVersion(map[string]string{"catalog.cattle.io/kube-version": tt.value})
			require.Equal(t, tt.want, got)
		})
	}

	require.Empty(t, checkKubeVersion(nil))
}
