package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	helmregistry "helm.sh/helm/v4/pkg/registry"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// ErrInvalidOptions is returned when client options contradict each other.
var ErrInvalidOptions = errors.New("invalid client options")

const (
	tlsHandshakeTimeout = 10 * time.Second
	requestTimeout      = 5 * time.Minute
)

// ClientOptions configures the connection to a registry.
type ClientOptions struct {
	// Username and Password authenticate with basic auth. Both or neither.
	Username string
	Password string
	// PlainHTTP talks HTTP instead of HTTPS.
	PlainHTTP bool
	// CAFile is a PEM file with extra CA certificates to trust, for
	// registries behind a private or self-signed CA.
	CAFile string
}

// NewClient returns a Client for a registry. Only the credentials in opts are
// used: nothing is read from Helm's or Docker's credential stores.
func NewClient(opts ClientOptions) (Client, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	httpClient, err := newHTTPClient(opts.CAFile)
	if err != nil {
		return nil, err
	}

	clientOpts := []helmregistry.ClientOption{
		helmregistry.ClientOptHTTPClient(httpClient),
		helmregistry.ClientOptAuthorizer(newAuthorizer(httpClient, opts.credential())),
	}
	if opts.PlainHTTP {
		clientOpts = append(clientOpts, helmregistry.ClientOptPlainHTTP())
	}

	c, err := helmregistry.NewClient(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("create registry client: %w", err)
	}

	return helmClient{client: c, opts: opts}, nil
}

// validate rejects contradictory options.
func (o ClientOptions) validate() error {
	if (o.Username == "") != (o.Password == "") {
		return fmt.Errorf("%w: username and password must be given together", ErrInvalidOptions)
	}
	if o.PlainHTTP && o.CAFile != "" {
		return fmt.Errorf("%w: a CA file cannot be used with plain HTTP", ErrInvalidOptions)
	}
	return nil
}

// credential returns the basic-auth credential in o, or the empty credential
// for anonymous access.
func (o ClientOptions) credential() auth.Credential {
	if o.Username == "" {
		return auth.EmptyCredential
	}
	return auth.Credential{Username: o.Username, Password: o.Password}
}

// newHTTPClient returns an HTTP client that trusts the system roots plus
// caFile, honors the proxy environment and times out requests.
func newHTTPClient(caFile string) (*http.Client, error) {
	tlsConfig, err := tlsConfigWithCA(caFile)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: tlsHandshakeTimeout,
			ForceAttemptHTTP2:   true,
		},
	}, nil
}

// newAuthorizer answers every registry challenge with credential. Nothing is
// read from Helm's or Docker's credential stores.
func newAuthorizer(httpClient *http.Client, credential auth.Credential) auth.Client {
	return auth.Client{
		Client:     httpClient,
		Credential: func(context.Context, string) (auth.Credential, error) { return credential, nil },
	}
}

// tlsConfigWithCA trusts the system roots plus the certificates in caFile.
// With no caFile it returns a config using the system roots only.
func tlsConfigWithCA(caFile string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return cfg, nil
	}

	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%w: %s holds no PEM certificate", ErrInvalidOptions, caFile)
	}
	cfg.RootCAs = pool
	return cfg, nil
}

// helmClient adapts Helm's registry client to Client.
type helmClient struct {
	client *helmregistry.Client
	opts   ClientOptions
}

var _ Client = helmClient{}

// Push uploads data with strict mode, which makes Helm check that ref ends in
// the chart's own name and version.
func (h helmClient) Push(data []byte, ref string) (string, error) {
	res, err := h.client.Push(data, ref, helmregistry.PushOptStrictMode(true))
	if err != nil {
		if kind, ok := Classify(err, h.opts); ok {
			return "", &RegistryError{Kind: kind, Err: err}
		}
		return "", err
	}
	return res.Ref, nil
}
