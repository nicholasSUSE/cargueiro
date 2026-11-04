package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// credentials are the registry credentials a command will use.
type credentials struct {
	Username string
	Password string
	// Stored is true when they were saved by `registry login`.
	Stored bool
}

// defaultStore opens the credentials file in the user's config directory.
func defaultStore() (*auth.Store, error) {
	path, err := auth.DefaultPath()
	if err != nil {
		return nil, err
	}
	return auth.Open(path, auth.Options{UseNativeStore: true})
}

// resolveCredentials picks the credentials for host: the ones given on the
// command line or in the environment when there are any, otherwise the ones
// saved for exactly this host, otherwise none (anonymous).
func resolveCredentials(ctx context.Context, opts *rootOptions, host, username, password string) (credentials, error) {
	if username != "" || password != "" {
		return credentials{Username: username, Password: password}, nil
	}

	store, err := opts.openStore()
	if err != nil {
		return credentials{}, fmt.Errorf("open the credentials file: %w", err)
	}

	saved, found, err := store.Get(ctx, host)
	if err != nil {
		return credentials{}, err
	}
	if !found {
		return credentials{}, nil
	}

	opts.log.DebugContext(ctx, "using saved credentials", "host", host, "user", saved.Username)
	if err := store.CheckPermissions(); err != nil {
		warnf(opts.stderr, "%v", err)
	}
	return credentials{Username: saved.Username, Password: saved.Password, Stored: true}, nil
}

// registryHost parses the host a login or logout applies to. Credentials
// belong to a host, so an address with a path is rejected.
func registryHost(arg string) (string, error) {
	target, err := oci.ParseTarget(arg)
	if err != nil {
		return "", err
	}
	if target.Path != "" {
		return "", fmt.Errorf("%w: credentials belong to a host, not a path: use %q", oci.ErrInvalidRegistry, target.Host)
	}
	return target.Host, nil
}

// warnf prints a warning to w. A warning that cannot be written is not worth
// failing the command for.
func warnf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "warning: "+format+"\n", args...)
}
