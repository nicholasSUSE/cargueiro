package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/version"
)

// listFunc returns the tags of the chart at repo.
type listFunc func(ctx context.Context, repo oci.ChartRepo, opts oci.FetchOptions) ([]string, error)

// listedVersion is one version in the JSON output, with the tag the registry
// stores it under.
type listedVersion struct {
	Version string `json:"version"`
	Tag     string `json:"tag"`
}

func newListCmd(opts *rootOptions, list listFunc) (*cobra.Command, error) {
	var output string
	var plainHTTP bool
	var creds credentialFlags

	cmd := &cobra.Command{
		Use:  "list <registry/path/chart>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return listVersions(cmd, opts, list, args[0], output, plainHTTP, &creds)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&output, "output", "o", outputText, "output format: text or json")
	flags.BoolVar(&plainHTTP, "plain-http", false, "talk HTTP to the registry instead of HTTPS, for local registries")
	creds.addFlags(flags)
	if err := applyDoc(cmd, "list"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// listVersions prints the versions of the chart at reference, newest first.
func listVersions(cmd *cobra.Command, opts *rootOptions, list listFunc, reference, output string, plainHTTP bool, creds *credentialFlags) error {
	if err := checkOutputFormat(output); err != nil {
		return err
	}
	repo, err := oci.ParseChartRepo(reference)
	if err != nil {
		return err
	}

	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	rule, err := cfg.Find(repo.Name())
	if err != nil {
		return err
	}
	versions := cfg.Versions()
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "listing versions", "repository", repo.String(), "output", output)

	given, err := creds.read(cmd, opts)
	if err != nil {
		return err
	}
	fetch, err := given.credentialFunc(cmd.Context()).FetchOptions(repo.Host, plainHTTP)
	if err != nil {
		return err
	}
	tags, err := list(cmd.Context(), repo, fetch)
	if err != nil {
		return err
	}
	found := make([]string, len(tags))
	for i, tag := range tags {
		found[i] = version.FromOCITag(tag)
	}
	sorted, err := policy.SortNewestFirst(found, rule.PlainVersion)
	if err != nil {
		return fmt.Errorf("order the versions of %s: %w", repo, err)
	}

	if err := printVersions(cmd.OutOrStdout(), sorted, output); err != nil {
		return fmt.Errorf("write versions: %w", err)
	}
	return nil
}

// printVersions writes versions as one per line, or as a JSON list.
func printVersions(w io.Writer, versions []string, output string) error {
	if output == outputJSON {
		listed := make([]listedVersion, len(versions))
		for i, v := range versions {
			listed[i] = listedVersion{Version: v, Tag: version.ToOCITag(v)}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(listed)
	}

	var b strings.Builder
	for _, v := range versions {
		b.WriteString(v + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
