package oci

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Target
		wantErr string
	}{
		{name: "host only", in: "registry.example.com", want: Target{Host: "registry.example.com"}},
		{name: "host and port", in: "127.0.0.1:5050", want: Target{Host: "127.0.0.1:5050"}},
		{name: "host, port and path", in: "127.0.0.1:5050/rancher/charts", want: Target{Host: "127.0.0.1:5050", Path: "rancher/charts"}},
		{name: "oci scheme", in: "oci://registry.example.com/team", want: Target{Host: "registry.example.com", Path: "team"}},
		{name: "https scheme", in: "https://registry.example.com/team", want: Target{Host: "registry.example.com", Path: "team"}},
		{name: "trailing slashes", in: "registry.example.com/team//", want: Target{Host: "registry.example.com", Path: "team"}},
		{name: "surrounding space", in: "  registry.example.com  ", want: Target{Host: "registry.example.com"}},

		{name: "empty", in: "", wantErr: "empty address"},
		{name: "only a scheme", in: "oci://", wantErr: "empty address"},
		{name: "no host", in: "/rancher/charts", wantErr: "no host"},
		{name: "no host with port", in: ":5000/charts", wantErr: "no host"},
		{name: "non numeric port", in: "host:abc/charts", wantErr: "invalid port"},
		{name: "port out of range", in: "host:70000", wantErr: "invalid port"},
		{name: "empty port", in: "host:/charts", wantErr: "invalid port"},
		{name: "double slash inside path", in: "host/a//b", wantErr: "invalid path"},
		{name: "dot dot in path", in: "host/a/../b", wantErr: "invalid path"},
		{name: "dot in path", in: "host/./b", wantErr: "invalid path"},
		{name: "tag in path", in: "host/charts:1.0", wantErr: "must not contain ':'"},
		{name: "credentials in address", in: "user:pass@host/charts", wantErr: "forbidden characters"},
		{name: "space inside", in: "host/my charts", wantErr: "forbidden characters"},
		{name: "query string", in: "host/charts?x=1", wantErr: "forbidden characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTarget(tt.in)

			if tt.wantErr != "" {
				require.ErrorIs(t, err, ErrInvalidRegistry)
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestTargetRef(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		want   string
	}{
		{name: "without path", target: Target{Host: "h:5000"}, want: "h:5000/fleet:1.0.0+up1"},
		{name: "with path", target: Target{Host: "h:5000", Path: "rancher/charts"}, want: "h:5000/rancher/charts/fleet:1.0.0+up1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.target.Ref("fleet", "1.0.0+up1"))
		})
	}
}
