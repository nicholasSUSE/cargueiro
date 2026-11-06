package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckPermitsOS(t *testing.T) {
	const form = `must list "linux", and optionally "windows", separated by commas`

	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "linux", value: "linux"},
		{name: "linux and windows", value: "linux,windows"},
		{name: "windows and linux", value: "windows,linux"},
		{name: "surrounding space is ignored", value: "  linux,windows "},

		{name: "absent is left to checkMissing", value: ""},
		{name: "blank is left to checkMissing", value: "  "},

		{name: "windows alone", value: "windows", want: []string{`catalog.cattle.io/permits-os "windows": must include "linux"`}},
		{name: "unknown system", value: "linux,darwin", want: []string{`catalog.cattle.io/permits-os "linux,darwin": ` + form}},
		{name: "space after the comma", value: "linux, windows", want: []string{`catalog.cattle.io/permits-os "linux, windows": ` + form}},
		{name: "trailing comma", value: "linux,", want: []string{`catalog.cattle.io/permits-os "linux,": ` + form}},
		{name: "leading comma", value: ",linux", want: []string{`catalog.cattle.io/permits-os ",linux": ` + form}},
		{name: "different case", value: "Linux", want: []string{`catalog.cattle.io/permits-os "Linux": ` + form}},
		{name: "linux twice", value: "linux,linux", want: []string{`catalog.cattle.io/permits-os "linux,linux": "linux" is listed twice`}},
		{name: "windows twice", value: "linux,windows,windows", want: []string{`catalog.cattle.io/permits-os "linux,windows,windows": "windows" is listed twice`}},
		{name: "separated by something else", value: "linux;windows", want: []string{`catalog.cattle.io/permits-os "linux;windows": ` + form}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkPermitsOS(map[string]string{"catalog.cattle.io/permits-os": tt.value})
			require.Equal(t, tt.want, got)
		})
	}

	require.Empty(t, checkPermitsOS(nil))
	require.Empty(t, checkPermitsOS(map[string]string{"catalog.rancher.io/permits-os": "windows"}))
}
