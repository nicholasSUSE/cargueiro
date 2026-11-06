package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckRancherVersion(t *testing.T) {
	const form = `must look like ">= 2.15.0-0 < 2.16.0-0"`

	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "valid", value: ">= 2.15.0-0 < 2.16.0-0"},
		{name: "valid with a wider window", value: ">= 2.12.0-0 < 2.15.0-0"},
		{name: "valid without space after the operator", value: ">=2.15.0-0 < 2.16.0-0"},
		{name: "valid with surrounding space", value: "  >= 2.15.0-0 < 2.16.0-0 "},
		{name: "numeric comparison, not text", value: ">= 2.9.0-0 < 2.10.0-0"},
		{name: "patch decides when major and minor are equal", value: ">= 2.15.1-0 < 2.15.2-0"},
		{name: "patch decides against the range", value: ">= 2.15.2-0 < 2.15.1-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.15.2-0 < 2.15.1-0": lower bound 2.15.2-0 must be below upper bound 2.15.1-0`}},
		{name: "a number too large for an integer", value: ">= 2.99999999999999999999.0-0 < 2.16.0-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.99999999999999999999.0-0 < 2.16.0-0": version number "99999999999999999999" is too large`}},
		{name: "an upper bound too large for an integer", value: ">= 2.15.0-0 < 2.16.99999999999999999999-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.15.0-0 < 2.16.99999999999999999999-0": version number "99999999999999999999" is too large`}},

		{name: "absent is left to checkMissing", value: ""},
		{name: "blank is left to checkMissing", value: "  "},

		{name: "upper bound missing", value: ">= 2.11.0-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.11.0-0": ` + form}},
		{name: "lower bound missing", value: "< 2.16.0-0", want: []string{`catalog.cattle.io/rancher-version "< 2.16.0-0": ` + form}},
		{name: "prerelease suffix missing", value: ">= 2.15.0 < 2.16.0", want: []string{`catalog.cattle.io/rancher-version ">= 2.15.0 < 2.16.0": ` + form}},
		{name: "prerelease suffix on one side only", value: ">= 2.15.0-0 < 2.16.0", want: []string{`catalog.cattle.io/rancher-version ">= 2.15.0-0 < 2.16.0": ` + form}},
		{name: "other prerelease", value: ">= 2.15.0-rc1 < 2.16.0-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.15.0-rc1 < 2.16.0-0": ` + form}},
		{name: "partial version", value: ">= 2.15-0 < 2.16.0-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.15-0 < 2.16.0-0": ` + form}},
		{name: "inclusive upper bound", value: ">= 2.15.0-0 <= 2.16.0-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.15.0-0 <= 2.16.0-0": ` + form}},
		{name: "union", value: ">= 2.12.0-0 < 2.13.0-0 || >= 2.15.0-0 < 2.16.0-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.12.0-0 < 2.13.0-0 || >= 2.15.0-0 < 2.16.0-0": ` + form}},
		{name: "text", value: "latest", want: []string{`catalog.cattle.io/rancher-version "latest": ` + form}},

		{name: "lower above upper", value: ">= 2.16.0-0 < 2.15.0-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.16.0-0 < 2.15.0-0": lower bound 2.16.0-0 must be below upper bound 2.15.0-0`}},
		{name: "lower equals upper", value: ">= 2.15.0-0 < 2.15.0-0", want: []string{`catalog.cattle.io/rancher-version ">= 2.15.0-0 < 2.15.0-0": lower bound 2.15.0-0 must be below upper bound 2.15.0-0`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkRancherVersion(map[string]string{"catalog.cattle.io/rancher-version": tt.value})
			require.Equal(t, tt.want, got)
		})
	}

	require.Empty(t, checkRancherVersion(nil))
}
