package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// Kind classifies a failed registry request by its likely cause, so callers
// can offer advice without parsing error text.
type Kind int

// The kinds of failure Classify recognizes.
const (
	// KindAuthRequired: the registry wants credentials and none were given.
	KindAuthRequired Kind = iota + 1
	// KindAuthRejected: the registry refused the given credentials.
	KindAuthRejected
	// KindForbidden: the credentials are valid but may not do this.
	KindForbidden
	// KindTLSRequired: plain HTTP was used against a TLS-only endpoint.
	KindTLSRequired
	// KindHTTPOnly: HTTPS was used against a plain-HTTP endpoint.
	KindHTTPOnly
	// KindUntrustedCertificate: the server certificate is not signed by a trusted CA.
	KindUntrustedCertificate
	// KindCertificateName: the server certificate is for a different host name.
	KindCertificateName
	// KindUnreachable: nothing accepted the connection.
	KindUnreachable
	// KindUnknownHost: the host name does not resolve.
	KindUnknownHost
	// KindTimeout: the request took too long.
	KindTimeout
)

// RegistryError is a failed registry request with a recognized cause.
type RegistryError struct {
	Kind Kind
	Err  error
}

func (e *RegistryError) Error() string { return e.Err.Error() }

func (e *RegistryError) Unwrap() error { return e.Err }

// Classify reports the likely cause of err, a failure from a registry request
// made with opts. It returns false when the cause is not recognized.
func Classify(err error, opts ClientOptions) (Kind, bool) {
	if err == nil {
		return 0, false
	}

	if errors.Is(err, auth.ErrBasicCredentialNotFound) {
		return KindAuthRequired, true
	}

	var resp *errcode.ErrorResponse
	if errors.As(err, &resp) {
		if kind, ok := classifyStatus(resp.StatusCode, opts); ok {
			return kind, true
		}
	}

	var (
		unknownAuthority x509.UnknownAuthorityError
		hostname         x509.HostnameError
		certInvalid      x509.CertificateInvalidError
		verification     *tls.CertificateVerificationError
		dnsErr           *net.DNSError
	)
	switch {
	case errors.As(err, &hostname):
		return KindCertificateName, true
	case errors.As(err, &unknownAuthority), errors.As(err, &certInvalid), errors.As(err, &verification):
		return KindUntrustedCertificate, true
	case !opts.PlainHTTP && strings.Contains(err.Error(), "server gave HTTP response to HTTPS client"):
		return KindHTTPOnly, true
	case errors.Is(err, syscall.ECONNREFUSED):
		return KindUnreachable, true
	case errors.As(err, &dnsErr):
		return KindUnknownHost, true
	case isTimeout(err):
		return KindTimeout, true
	}

	return 0, false
}

func classifyStatus(status int, opts ClientOptions) (Kind, bool) {
	switch {
	case status == http.StatusUnauthorized && opts.Username == "":
		return KindAuthRequired, true
	case status == http.StatusUnauthorized:
		return KindAuthRejected, true
	case status == http.StatusForbidden:
		return KindForbidden, true
	case status == http.StatusBadRequest && opts.PlainHTTP:
		// A TLS-only server answers plain HTTP with 400.
		return KindTLSRequired, true
	}
	return 0, false
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
