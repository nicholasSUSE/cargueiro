package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/auth"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// verifyFunc checks credentials against the registry at host.
type verifyFunc func(ctx context.Context, host string, opts oci.ClientOptions) (verified bool, err error)

func newLoginCmd(opts *rootOptions, verify verifyFunc) (*cobra.Command, error) {
	var (
		username      string
		password      string
		passwordStdin bool
		plainHTTP     bool
		caFile        string
	)

	cmd := &cobra.Command{
		Use:  "login <host>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			host, err := registryHost(args[0])
			if err != nil {
				return err
			}
			store, err := opts.openStore()
			if err != nil {
				return err
			}
			existing, loggedIn, err := store.Load()
			if err != nil {
				return err
			}
			if loggedIn {
				return fmt.Errorf("already logged in to %s: run `cargueiro registry logout` first", existing.Host)
			}

			cred, err := loginCredentials(cmd, opts, username, password, passwordStdin)
			if err != nil {
				return err
			}

			caPEM, err := readCA(caFile)
			if err != nil {
				return err
			}
			clientOpts := oci.ClientOptions{
				Username: cred.Username, Password: cred.Password, PlainHTTP: plainHTTP, CAPEM: caPEM,
			}
			verified, err := verify(ctx, host, clientOpts)
			if errors.Is(err, oci.ErrInvalidOptions) {
				return err
			}
			if err != nil {
				return fmt.Errorf("login to %s failed: %w", host, err)
			}
			if !verified {
				warnf(opts.stderr, "%s accepts anonymous requests, so these credentials could not be checked", host)
			}

			err = store.Save(auth.Login{
				Host: host, Username: cred.Username, Password: cred.Password, PlainHTTP: plainHTTP, CAPEM: string(caPEM),
			})
			if err != nil {
				return err
			}
			warnf(opts.stderr, "the password is saved in plain text in %s, readable only by you", store.Path())

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Login succeeded for %s\n", host)
			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&username, "username", "u", "", "registry username (default: $"+envUsername+", else asked for)")
	flags.StringVar(&password, "password", "", "registry password; insecure, prefer --password-stdin or the prompt")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "read the password from standard input")
	flags.BoolVar(&plainHTTP, "plain-http", false, "reach the registry over HTTP instead of HTTPS, and remember that")
	flags.StringVar(&caFile, "ca-file", "", "PEM file with extra CA certificates to trust, remembered with the login")
	cmd.MarkFlagsMutuallyExclusive("password", "password-stdin")

	if err := applyDoc(cmd, "registry-login"); err != nil {
		return nil, err
	}
	return cmd, nil
}

// loginCredentials gets the username and password for a login. The username
// comes from --username, the environment or a prompt. The password comes from
// --password, standard input, the environment or a prompt, in that order.
func loginCredentials(cmd *cobra.Command, opts *rootOptions, username, password string, fromStdin bool) (credentials, error) {
	if username == "" {
		username = opts.getenv(envUsername)
	}
	if username == "" {
		switch {
		case fromStdin:
			return credentials{}, errors.New("a username is required: give --username, because standard input carries the password")
		case !opts.prompt.Interactive():
			return credentials{}, fmt.Errorf("a username is required: give --username or set $%s", envUsername)
		}
		var err error
		if username, err = opts.prompt.Line("Username: "); err != nil {
			return credentials{}, err
		}
	}

	switch {
	case password != "":
		warnf(opts.stderr, "--password on the command line is insecure: it stays in your shell history "+
			"and is visible in the process list; use --password-stdin or the prompt")
	case fromStdin:
		var err error
		if password, err = readPassword(cmd.InOrStdin(), true, opts.getenv); err != nil {
			return credentials{}, err
		}
	default:
		password = opts.getenv(envPassword)
		if password == "" && opts.prompt.Interactive() {
			var err error
			if password, err = opts.prompt.Secret("Password: "); err != nil {
				return credentials{}, err
			}
		}
	}
	if password == "" {
		return credentials{}, fmt.Errorf("a password is required: use --password-stdin, set $%s, or run in a terminal", envPassword)
	}

	return credentials{Username: username, Password: password}, nil
}

// readCA reads the CA certificates in path, or returns nil when path is empty.
func readCA(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}

	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	return pem, nil
}
