package cmd

import (
	"context"
	"errors"
	"fmt"

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
				return fmt.Errorf("open the credentials file: %w", err)
			}

			cred, err := loginCredentials(cmd, opts, username, password, passwordStdin)
			if err != nil {
				return err
			}

			clientOpts := oci.ClientOptions{
				Username: cred.Username, Password: cred.Password, PlainHTTP: plainHTTP, CAFile: caFile,
			}
			verified, err := verify(ctx, host, clientOpts)
			if err != nil {
				hints := hintSet{}
				hints.add(hintFor(err, hintContext{
					username: cred.Username, caFile: caFile, host: host, address: "the registry address",
				}))
				return fmt.Errorf("login to %s failed: %w%s", host, err, &hints)
			}
			if !verified {
				warnf(opts.stderr, "%s accepts anonymous requests, so these credentials could not be checked", host)
			}

			if err := store.CheckPermissions(); errors.Is(err, auth.ErrInsecurePermissions) {
				warnf(opts.stderr, "%v; it is now restricted to your user", err)
			}

			plaintext, err := store.Put(ctx, host, cred)
			if err != nil {
				return err
			}
			if plaintext {
				warnf(opts.stderr, "the password is stored unencrypted (base64) in %s; "+
					"install a Docker credential helper to keep it in a keychain", store.Path())
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Login succeeded for %s\n", host)
			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&username, "username", "u", "", "registry username (default: $"+envUsername+", else asked for)")
	flags.StringVar(&password, "password", "", "registry password; insecure, prefer --password-stdin or the prompt")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "read the password from standard input")
	flags.BoolVar(&plainHTTP, "plain-http", false, "use HTTP instead of HTTPS to check the login")
	flags.StringVar(&caFile, "ca-file", "", "PEM file with extra CA certificates to trust")
	cmd.MarkFlagsMutuallyExclusive("password", "password-stdin")

	if err := applyDoc(cmd, "registry-login"); err != nil {
		return nil, err
	}
	return cmd, nil
}

// loginCredentials gets the username and password for a login. The username
// comes from --username, the environment or a prompt. The password comes from
// --password, standard input, the environment or a prompt, in that order.
func loginCredentials(cmd *cobra.Command, opts *rootOptions, username, password string, fromStdin bool) (auth.Credential, error) {
	if username == "" {
		username = opts.getenv(envUsername)
	}
	if username == "" {
		switch {
		case fromStdin:
			return auth.Credential{}, errors.New("a username is required: give --username, because standard input carries the password")
		case !opts.prompt.Interactive():
			return auth.Credential{}, fmt.Errorf("a username is required: give --username or set $%s", envUsername)
		}
		var err error
		if username, err = opts.prompt.Line("Username: "); err != nil {
			return auth.Credential{}, err
		}
	}

	switch {
	case password != "":
		warnf(opts.stderr, "--password on the command line is insecure: it stays in your shell history "+
			"and is visible in the process list; use --password-stdin or the prompt")
	case fromStdin:
		var err error
		if password, err = readPassword(cmd.InOrStdin(), true, opts.getenv); err != nil {
			return auth.Credential{}, err
		}
	default:
		password = opts.getenv(envPassword)
		if password == "" && opts.prompt.Interactive() {
			var err error
			if password, err = opts.prompt.Secret("Password: "); err != nil {
				return auth.Credential{}, err
			}
		}
	}
	if password == "" {
		return auth.Credential{}, fmt.Errorf("a password is required: use --password-stdin, set $%s, or run in a terminal", envPassword)
	}

	return auth.Credential{Username: username, Password: password}, nil
}
