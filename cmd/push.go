package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
)

const (
	envUsername = "CARGUEIRO_USERNAME"
	envPassword = "CARGUEIRO_PASSWORD"
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
		registry string
		creds    credentialFlags
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

			given := creds.read(opts)
			conn, err := given.connection(cmd.Context(), target.Host)
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
	creds.addFlags(flags)
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
