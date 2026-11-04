package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// timeoutError is a net.Error that reports a timeout.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return false }

func status(code int) error { return &errcode.ErrorResponse{StatusCode: code} }

func TestClassify(t *testing.T) {
	anonymous := ClientOptions{}
	withUser := ClientOptions{Username: "u", Password: "p"}
	plain := ClientOptions{PlainHTTP: true}

	tests := []struct {
		name string
		err  error
		opts ClientOptions
		want Kind
	}{
		{name: "no credentials available", err: auth.ErrBasicCredentialNotFound, opts: anonymous, want: KindAuthRequired},
		{name: "401 without credentials", err: status(http.StatusUnauthorized), opts: anonymous, want: KindAuthRequired},
		{name: "401 with credentials", err: status(http.StatusUnauthorized), opts: withUser, want: KindAuthRejected},
		{name: "403", err: status(http.StatusForbidden), opts: withUser, want: KindForbidden},
		{name: "400 over plain HTTP", err: status(http.StatusBadRequest), opts: plain, want: KindTLSRequired},
		{name: "unknown authority", err: x509.UnknownAuthorityError{}, opts: anonymous, want: KindUntrustedCertificate},
		{name: "certificate verification error", err: &tls.CertificateVerificationError{Err: errors.New("bad")}, opts: anonymous, want: KindUntrustedCertificate},
		{name: "expired certificate", err: x509.CertificateInvalidError{Reason: x509.Expired}, opts: anonymous, want: KindUntrustedCertificate},
		{name: "certificate for another host", err: x509.HostnameError{Host: "reg.local"}, opts: anonymous, want: KindCertificateName},
		{name: "HTTP server answering HTTPS", err: errors.New(`Head "https://reg": http: server gave HTTP response to HTTPS client`), opts: anonymous, want: KindHTTPOnly},
		{name: "connection refused", err: &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, opts: anonymous, want: KindUnreachable},
		{name: "unknown host", err: &net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true}, opts: anonymous, want: KindUnknownHost},
		{name: "deadline exceeded", err: context.DeadlineExceeded, opts: anonymous, want: KindTimeout},
		{name: "network timeout", err: &net.OpError{Op: "read", Err: timeoutError{}}, opts: anonymous, want: KindTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Classify(tt.err, tt.opts)
			require.True(t, ok)
			require.Equal(t, tt.want, got)

			wrapped := fmt.Errorf("failed to perform %q on destination: %w", "Exists", tt.err)
			got, ok = Classify(wrapped, tt.opts)
			require.True(t, ok, "must still be recognized when wrapped")
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClassifyUnrecognized(t *testing.T) {
	tests := []struct {
		name string
		err  error
		opts ClientOptions
	}{
		{name: "nil", err: nil},
		{name: "unrelated error", err: errors.New("boom")},
		{name: "400 over HTTPS is not a TLS problem", err: status(http.StatusBadRequest)},
		{name: "404", err: status(http.StatusNotFound)},
		{name: "500", err: status(http.StatusInternalServerError)},
		{name: "HTTPS answer is only a problem without --plain-http", err: errors.New("server gave HTTP response to HTTPS client"), opts: ClientOptions{PlainHTTP: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := Classify(tt.err, tt.opts)
			require.False(t, ok)
		})
	}
}

func TestRegistryErrorKeepsTheCause(t *testing.T) {
	cause := fmt.Errorf("wrapped: %w", x509.UnknownAuthorityError{})
	err := fmt.Errorf("push a.tgz: %w", &RegistryError{Kind: KindUntrustedCertificate, Err: cause})

	var registryErr *RegistryError
	require.ErrorAs(t, err, &registryErr)
	require.Equal(t, KindUntrustedCertificate, registryErr.Kind)
	require.Equal(t, cause.Error(), registryErr.Error(), "the message is the original error's")
	var unknown x509.UnknownAuthorityError
	require.ErrorAs(t, err, &unknown, "the original error stays reachable")
}
