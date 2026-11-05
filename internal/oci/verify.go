package oci

import (
	"context"
	"fmt"
	"net/http"

	"oras.land/oras-go/v2/registry/remote"
)

// VerifyLogin checks the credentials in opts against the registry at host
// (host[:port], no path) by pinging its API.
//
// verified is true when the registry demanded credentials and accepted them.
// It is false, with a nil error, when the registry answers anonymous requests:
// then nothing proves the credentials are right. A rejection is returned as an
// error.
func VerifyLogin(ctx context.Context, host string, opts ClientOptions) (verified bool, err error) {
	if err := opts.validate(); err != nil {
		return false, err
	}
	if opts.Username == "" {
		return false, fmt.Errorf("%w: a login needs a username and a password", ErrInvalidOptions)
	}

	httpClient, err := newHTTPClient(opts.CAPEM)
	if err != nil {
		return false, err
	}

	anonymous := ClientOptions{PlainHTTP: opts.PlainHTTP, CAPEM: opts.CAPEM}
	if err := ping(ctx, host, httpClient, anonymous); err == nil {
		return false, nil
	}

	if err := ping(ctx, host, httpClient, opts); err != nil {
		return false, fmt.Errorf("ping %s: %w", host, err)
	}
	return true, nil
}

// ping calls the registry's API root with the credential in opts.
func ping(ctx context.Context, host string, httpClient *http.Client, opts ClientOptions) error {
	authorizer := newAuthorizer(httpClient, opts.credential())

	reg, err := remote.NewRegistry(host)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRegistry, err)
	}
	reg.Client = &authorizer
	reg.PlainHTTP = opts.PlainHTTP

	return reg.Ping(ctx)
}
