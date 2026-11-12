package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	helmregistry "helm.sh/helm/v4/pkg/registry"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/rancher/cargueiro/internal/version"
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
	// CAPEM holds extra CA certificates to trust, in PEM, for registries
	// behind a private or self-signed CA.
	CAPEM []byte
}

// NewClient returns a Client for a registry. Only the credentials in opts are
// used: nothing is read from Helm's or Docker's credential stores.
func NewClient(opts ClientOptions) (Client, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	httpClient, err := newHTTPClient(opts.CAPEM)
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
	if o.PlainHTTP && len(o.CAPEM) > 0 {
		return fmt.Errorf("%w: a CA certificate cannot be used with plain HTTP", ErrInvalidOptions)
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
// caPEM, honors the proxy environment and times out requests.
func newHTTPClient(caPEM []byte) (*http.Client, error) {
	tlsConfig, err := tlsConfigWithCA(caPEM)
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

// tlsConfigWithCA trusts the system roots plus the certificates in caPEM.
// With no caPEM it returns a config using the system roots only.
func tlsConfigWithCA(caPEM []byte) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if len(caPEM) == 0 {
		return cfg, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("%w: the CA certificate holds no PEM certificate", ErrInvalidOptions)
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
		return "", err
	}
	return res.Ref, nil
}

// Exists reports whether ref is already stored in its registry. A reference
// the registry does not know is not an error. Helm's resolve does not convert
// the version to a tag as its push does, so Exists does.
func (h helmClient) Exists(ref string) (bool, error) {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		ref = ref[:i+1] + version.ToOCITag(ref[i+1:])
	}

	if _, err := h.client.Resolve(ref); err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ChartDigest returns the digest of the chart layer of the chart stored at ref,
// which is the sha256 of the archive that was pushed. It reads the manifest
// with the credentials and the CA the client was made with.
func (h helmClient) ChartDigest(ref string) (string, error) {
	chartRef, err := ParseChartRef(ref)
	if err != nil {
		return "", err
	}

	httpClient, err := newHTTPClient(h.opts.CAPEM)
	if err != nil {
		return "", err
	}
	repo, err := openRepository(chartRef, httpClient, h.opts.credential(), h.opts.PlainHTTP)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	layer, err := chartLayer(ctx, repo, chartRef.Tag, chartRef.String(), h.opts.Username != "")
	if err != nil {
		return "", err
	}
	return layer.Digest.String(), nil
}
