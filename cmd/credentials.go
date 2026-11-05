package cmd

import (
	"context"
	"errors"
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

// connection is how to reach a registry: who to authenticate as, and the
// transport settings saved at login.
type connection struct {
	credentials
	PlainHTTP bool
	CAPEM     []byte
}

// clientOptions turns the connection into oci client options.
func (c connection) clientOptions() oci.ClientOptions {
	return oci.ClientOptions{
		Username:  c.Username,
		Password:  c.Password,
		PlainHTTP: c.PlainHTTP,
		CAPEM:     c.CAPEM,
	}
}

// defaultStore opens the login file in the user's config directory.
func defaultStore() (*auth.Store, error) {
	path, err := auth.DefaultPath()
	if err != nil {
		return nil, err
	}
	return auth.Open(path), nil
}

// resolveConnection works out how to reach host. The saved login is only used
// for the registry it was made for. The transport settings come from it. The
// credentials are the ones given on the command line or in the environment
// when there are any, otherwise the saved ones, otherwise none (anonymous).
func resolveConnection(ctx context.Context, opts *rootOptions, host, username, password string) (connection, error) {
	store, err := opts.openStore()
	if err != nil {
		return connection{}, err
	}

	login, found, err := store.Load()
	if err != nil {
		return connection{}, err
	}

	var conn connection
	forThisHost := found && login.Matches(host)
	if forThisHost {
		conn.PlainHTTP = login.PlainHTTP
		if login.CAPEM != "" {
			conn.CAPEM = []byte(login.CAPEM)
		}
	}

	if username != "" || password != "" {
		switch {
		case username == "":
			return connection{}, fmt.Errorf("a username is required: give --username or set $%s", envUsername)
		case password == "":
			return connection{}, fmt.Errorf("a password is required: use --password-stdin or set $%s", envPassword)
		}
		conn.Username, conn.Password = username, password
		return conn, nil
	}
	if !forThisHost {
		return conn, nil
	}

	opts.log.DebugContext(ctx, "using the saved login", "host", host, "user", login.Username)
	if err := store.CheckPermissions(); err != nil {
		if !errors.Is(err, auth.ErrInsecurePermissions) {
			return connection{}, err
		}
		warnf(opts.stderr, "%v", err)
	}
	conn.Username, conn.Password, conn.Stored = login.Username, login.Password, true
	return conn, nil
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
