package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

func registryErr(kind oci.Kind) error {
	return fmt.Errorf("push a.tgz: %w", &oci.RegistryError{Kind: kind, Err: errors.New("underlying")})
}

func TestHintFor(t *testing.T) {
	tests := []struct {
		name string
		kind oci.Kind
		ctx  pushContext
		want []string // substrings the hint must contain
	}{
		{name: "credentials required", kind: oci.KindAuthRequired, want: []string{"--username", "CARGUEIRO_PASSWORD", "--password-stdin"}},
		{name: "credentials rejected", kind: oci.KindAuthRejected, ctx: pushContext{username: "admin"}, want: []string{`"admin"`, "password"}},
		{name: "forbidden as user", kind: oci.KindForbidden, ctx: pushContext{username: "admin"}, want: []string{`"admin"`, "permissions"}},
		{name: "forbidden anonymously", kind: oci.KindForbidden, want: []string{"anonymous", "--username"}},
		{name: "TLS expected", kind: oci.KindTLSRequired, want: []string{"drop --plain-http", "--ca-file"}},
		{name: "HTTP only", kind: oci.KindHTTPOnly, want: []string{"use --plain-http"}},
		{name: "untrusted certificate", kind: oci.KindUntrustedCertificate, want: []string{"--ca-file"}},
		{name: "untrusted despite a CA file", kind: oci.KindUntrustedCertificate, ctx: pushContext{caFile: "ca.crt"}, want: []string{"ca.crt", "--ca-file"}},
		{name: "certificate name", kind: oci.KindCertificateName, want: []string{"host name", "--registry"}},
		{name: "unreachable", kind: oci.KindUnreachable, want: []string{"--registry", "running"}},
		{name: "unknown host", kind: oci.KindUnknownHost, want: []string{"--registry", "resolved"}},
		{name: "timeout", kind: oci.KindTimeout, want: []string{"--registry", "network"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := hintFor(registryErr(tt.kind), tt.ctx)

			require.NotEmpty(t, hint)
			for _, want := range tt.want {
				require.Contains(t, hint, want)
			}
		})
	}
}

func TestHintForUnrecognizedErrors(t *testing.T) {
	require.Empty(t, hintFor(errors.New("boom"), pushContext{}))
	require.Empty(t, hintFor(nil, pushContext{}))
	require.Empty(t, hintFor(registryErr(oci.Kind(999)), pushContext{}), "an unknown kind gives no hint")
}

func TestHintSet(t *testing.T) {
	var hints hintSet
	require.Empty(t, hints.String())

	hints.add("")
	hints.add("first")
	hints.add("second")
	hints.add("first")

	require.Equal(t, "\nhint: first\nhint: second", hints.String(), "distinct, in first-seen order, empty ignored")
}
