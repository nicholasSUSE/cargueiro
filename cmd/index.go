package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/index"
	"github.com/nicholasSUSE/cargueiro/internal/rules"
)

func newIndexCmd(opts *rootOptions) (*cobra.Command, error) {
	var repoDir, registry string
	var plainHTTP bool

	cmd := &cobra.Command{
		Use:  "index",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if repoDir == "" {
				return errors.New("--repo is required: give the path of the charts repository")
			}
			cfg, err := rules.Parse(opts.rules)
			if err != nil {
				return err
			}
			opts.log.DebugContext(cmd.Context(), "writing index", "repo", repoDir)
			return index.Write(cmd.Context(), cfg, repoDir, index.Options{Registry: registry, PlainHTTP: plainHTTP})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&repoDir, "repo", "", "path of the charts repository, which holds entries/ and gets the index.yaml")
	flags.StringVar(&registry, "registry", "", "registry path the URLs of new versions must point to, host[:port]/path, instead of the staging registry of the chart")
	flags.BoolVar(&plainHTTP, "plain-http", false, "talk HTTP to the registry instead of HTTPS, for local registries")
	if err := applyDoc(cmd, "index"); err != nil {
		return nil, err
	}

	return cmd, nil
}
