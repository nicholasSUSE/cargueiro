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

// archivePusher pushes chart archives and returns the references they were
// stored under.
type archivePusher interface {
	Push(ctx context.Context, archive string) (string, error)
	// PushFamily pushes the archives of a family, in the order given, and says
	// what happened to each of them.
	PushFamily(ctx context.Context, archives []string, notice oci.RetryNotice) (*oci.FamilyResult, error)
}

// pusherFactory builds an archivePusher for a registry.
type pusherFactory func(target oci.Target, guard *oci.Guard, opts oci.ClientOptions) (archivePusher, error)

// newOCIPusher is the production pusherFactory.
func newOCIPusher(target oci.Target, guard *oci.Guard, opts oci.ClientOptions) (archivePusher, error) {
	client, err := oci.NewClient(opts)
	if err != nil {
		return nil, err
	}
	return oci.NewPusher(client, target, guard).WithRetry(oci.DefaultRetry()), nil
}

func newPushCmd(opts *rootOptions, newPusher pusherFactory) (*cobra.Command, error) {
	var (
		registry   string
		familyName string
		creds      credentialFlags
	)

	cmd := &cobra.Command{
		Use: "push <archive.tgz>",
		Args: func(cmd *cobra.Command, args []string) error {
			return checkArgs(familyName)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if familyName != "" {
				return pushFamily(cmd, opts, newPusher, familyName, args, registry, &creds)
			}
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
	flags.StringVar(&familyName, "family", "", "push every chart of this family, with one archive for each of its charts")
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
