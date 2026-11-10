package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
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
type pusherFactory func(target oci.Target, guard *oci.Guard, opts oci.ClientOptions) (archivePusher, error)

// newOCIPusher is the production pusherFactory.
func newOCIPusher(target oci.Target, guard *oci.Guard, opts oci.ClientOptions) (archivePusher, error) {
	client, err := oci.NewClient(opts)
	if err != nil {
		return nil, err
	}
	return oci.NewPusher(client, target, guard), nil
}

func newPushCmd(opts *rootOptions, newPusher pusherFactory) (*cobra.Command, error) {
	var (
		registry      string
		username      string
		passwordStdin bool
	)

	cmd := &cobra.Command{
		Use:  "push <archive.tgz>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := oci.ParseTarget(registry)
			if err != nil {
				return err
			}
			guard, err := newGuard(opts.rules)
			if err != nil {
				return err
			}
			// Before reading any secret or touching the network.
			if guard.Guarded(target) {
				name, _, err := oci.ReadIdentity(args[0])
				if err != nil {
					return err
				}
				if err := guard.Check(target, name); err != nil {
					return err
				}
			}

			if username == "" {
				username = opts.getenv(envUsername)
			}
			password, err := readPassword(cmd.InOrStdin(), passwordStdin, opts.getenv)
			if err != nil {
				return err
			}

			conn, err := resolveConnection(cmd.Context(), opts, target.Host, username, password)
			if err != nil {
				return err
			}

			pusher, err := newPusher(target, guard, conn.clientOptions())
			if err != nil {
				return err
			}

			archive := args[0]
			opts.log.DebugContext(cmd.Context(), "pushing chart", "archive", archive, "registry", registry)

			ref, err := pusher.Push(cmd.Context(), archive)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "oci://"+ref); err != nil {
				return fmt.Errorf("write reference: %w", err)
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&registry, "registry", "", "destination registry: host[:port][/path]")
	flags.StringVarP(&username, "username", "u", "", "registry username (default: $"+envUsername+")")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "read the password from standard input instead of $"+envPassword)
	if err := cmd.MarkFlagRequired("registry"); err != nil {
		return nil, fmt.Errorf("mark --registry required: %w", err)
	}

	if err := applyDoc(cmd, "push"); err != nil {
		return nil, err
	}
	return cmd, nil
}

// newGuard builds the push policy from the rules: where images go, where the
// charts of each edition go, and the edition of every listed chart.
func newGuard(data []byte) (*oci.Guard, error) {
	cfg, err := rules.Parse(data)
	if err != nil {
		return nil, err
	}

	registries := cfg.Registries()
	return oci.NewGuard(oci.Policy{
		Images: registries.ImageRepositories(),
		Charts: map[string][]string{
			rules.EditionRancherCharts: registries.ChartRepositories(rules.EditionRancherCharts),
			rules.EditionPrimeCharts:   registries.ChartRepositories(rules.EditionPrimeCharts),
		},
		Editions: cfg.Editions(),
	})
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
