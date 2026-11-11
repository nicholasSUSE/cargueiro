package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/ship"
)

// familyFlags say which family `ship` ships, instead of one reference: the
// references are made from the registry, the name of each chart of the family
// and the version.
type familyFlags struct {
	name     string
	version  string
	registry string
}

// addFlags registers --family, --version and --registry.
func (f *familyFlags) addFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.name, "family", "", "ship every chart of this family, at --version, from --registry")
	flags.StringVar(&f.version, "version", "", "the version of the family, as in its Chart.yaml files (with --family)")
	flags.StringVar(&f.registry, "registry", "", "where the charts were pushed, host[:port]/path with no chart name (with --family)")
}

// set reports whether a family was asked for.
func (f *familyFlags) set() bool {
	return f.name != ""
}

// checkArgs refuses a reference: with a family they are made from the flags.
func (f *familyFlags) checkArgs(args []string) error {
	if len(args) > 0 {
		return errors.New("give no reference with --family: the references are made from --registry and --version")
	}
	return nil
}

// check refuses --version and --registry without --family, and --family without
// them.
func (f *familyFlags) check() error {
	switch {
	case !f.set() && (f.version != "" || f.registry != ""):
		return errors.New("--version and --registry are only for --family")
	case f.set() && f.version == "":
		return errors.New("--family needs --version")
	case f.set() && f.registry == "":
		return errors.New("--family needs --registry")
	}
	return nil
}

// shipFamily writes the entry, and the logo if there is one, of every chart of
// the family to the charts repository in repoDir, and prints the files. The
// charts are all prepared before any is written, and then written together:
// nothing is written unless every chart of the family can be shipped.
func shipFamily(cmd *cobra.Command, opts *rootOptions, prepare prepareFunc, verifyCheckout verifyCheckoutFunc, family familyFlags, repoDir, branch string, plainHTTP, dryRun bool, creds *credentialFlags) error {
	if !filepath.IsAbs(repoDir) {
		return fmt.Errorf("--repo must be an absolute path, got %q", repoDir)
	}
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	members, err := cfg.Family(family.name)
	if err != nil {
		return err
	}
	refs := make([]oci.ChartRef, 0, len(members.Charts))
	for _, chart := range members.Charts {
		ref, err := oci.ParseChartRef(strings.TrimRight(family.registry, "/") + "/" + chart.Name + ":" + family.version)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
	}
	opts.log.DebugContext(cmd.Context(), "shipping family", "family", family.name, "version", family.version, "repo", repoDir, "branch", branch, "dryRun", dryRun)

	if err := verifyCheckout(cmd.Context(), repoDir, branch); err != nil {
		return err
	}

	given := creds.read(opts)
	shipments, err := prepareAll(cmd, cfg, prepare, members, refs, family.version, repoDir, ship.Options{PlainHTTP: plainHTTP, Credentials: given.credentialFunc(cmd.Context())})
	if err != nil {
		return err
	}

	if !dryRun {
		if err := ship.WriteAll(repoDir, shipments); err != nil {
			return err
		}
	}
	var paths []string
	for _, shipment := range shipments {
		paths = append(paths, shipment.Paths()...)
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), strings.Join(paths, "\n")); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

// prepareAll prepares the shipment of every chart of the family, in the order of
// the family, and reports every chart that cannot be shipped, not only the
// first. Every shipment must be of the version that was asked for.
func prepareAll(cmd *cobra.Command, cfg *rules.Config, prepare prepareFunc, members rules.Family, refs []oci.ChartRef, version, repoDir string, opts ship.Options) ([]ship.Shipment, error) {
	shipments := make([]ship.Shipment, 0, len(refs))
	var problems []error
	for i, ref := range refs {
		shipment, err := prepare(cmd.Context(), cfg, ref, repoDir, opts)
		var invalid *ship.InvalidChartError
		switch {
		case errors.As(err, &invalid):
			if _, werr := fmt.Fprint(cmd.ErrOrStderr(), resultText(invalid.Result)); werr != nil {
				return nil, fmt.Errorf("write result: %w", werr)
			}
			problems = append(problems, err)
		case err != nil:
			problems = append(problems, fmt.Errorf("%s: %w", members.Charts[i].Name, err))
		case shipment.Version != version:
			problems = append(problems, fmt.Errorf("%s: the chart is version %s, and the family is shipped at %s", members.Charts[i].Name, shipment.Version, version))
		default:
			shipments = append(shipments, shipment)
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return shipments, nil
}
