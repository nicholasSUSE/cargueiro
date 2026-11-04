package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

const (
	envUsername = "CARGUEIRO_USERNAME"
	envPassword = "CARGUEIRO_PASSWORD"

	// maxPasswordSize bounds what --password-stdin reads.
	maxPasswordSize = 4096
)

// archivePusher pushes one chart archive and returns the reference it was
// stored under.
type archivePusher interface {
	Push(ctx context.Context, archive string) (string, error)
}

// pusherFactory builds an archivePusher for a registry.
type pusherFactory func(target oci.Target, opts oci.ClientOptions) (archivePusher, error)

// newOCIPusher is the production pusherFactory.
func newOCIPusher(target oci.Target, opts oci.ClientOptions) (archivePusher, error) {
	client, err := oci.NewClient(opts)
	if err != nil {
		return nil, err
	}
	return oci.NewPusher(client, target), nil
}

func newPushCmd(opts *rootOptions, newPusher pusherFactory) (*cobra.Command, error) {
	var (
		registry      string
		username      string
		passwordStdin bool
		plainHTTP     bool
		caFile        string
	)

	cmd := &cobra.Command{
		Use:  "push <archive.tgz>...",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, archives []string) error {
			target, err := oci.ParseTarget(registry)
			if err != nil {
				return err
			}
			// Before reading any secret or touching the network.
			if err := oci.CheckTarget(target); err != nil {
				return err
			}

			if username == "" {
				username = opts.getenv(envUsername)
			}
			password, err := readPassword(cmd.InOrStdin(), passwordStdin, opts.getenv)
			if err != nil {
				return err
			}

			pusher, err := newPusher(target, oci.ClientOptions{
				Username:  username,
				Password:  password,
				PlainHTTP: plainHTTP,
				CAFile:    caFile,
			})
			if err != nil {
				return err
			}

			var (
				errs  []error
				hints hintSet
			)
			hintCtx := pushContext{username: username, caFile: caFile}
			for _, archive := range archives {
				opts.log.DebugContext(cmd.Context(), "pushing chart", "archive", archive, "registry", registry)

				ref, err := pusher.Push(cmd.Context(), archive)
				if err != nil {
					errs = append(errs, err)
					hints.add(hintFor(err, hintCtx))
					continue
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "oci://"+ref); err != nil {
					return fmt.Errorf("write reference: %w", err)
				}
			}

			if len(errs) > 0 {
				return fmt.Errorf("%d of %d archives failed:\n%w%s", len(errs), len(archives), errors.Join(errs...), &hints)
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&registry, "registry", "", "destination registry: host[:port][/path]")
	flags.StringVarP(&username, "username", "u", "", "registry username (default: $"+envUsername+")")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "read the password from standard input instead of $"+envPassword)
	flags.BoolVar(&plainHTTP, "plain-http", false, "use HTTP instead of HTTPS")
	flags.StringVar(&caFile, "ca-file", "", "PEM file with extra CA certificates to trust")
	if err := cmd.MarkFlagRequired("registry"); err != nil {
		return nil, fmt.Errorf("mark --registry required: %w", err)
	}

	if err := applyDoc(cmd, "push"); err != nil {
		return nil, err
	}
	return cmd, nil
}

// readPassword returns the registry password: from stdin when asked to,
// otherwise from the environment. It returns "" when none is set.
func readPassword(stdin io.Reader, fromStdin bool, getenv func(string) string) (string, error) {
	if !fromStdin {
		return getenv(envPassword), nil
	}

	data, err := io.ReadAll(io.LimitReader(stdin, maxPasswordSize+1))
	if err != nil {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	if len(data) > maxPasswordSize {
		return "", fmt.Errorf("password from stdin is longer than %d bytes", maxPasswordSize)
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}
