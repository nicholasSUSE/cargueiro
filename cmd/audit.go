package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/audit"
	"github.com/rancher/cargueiro/internal/rules"
)

func newAuditCmd(opts *rootOptions, fetch audit.FetchFunc) (*cobra.Command, error) {
	var repoDir, registry, output string
	var plainHTTP bool
	var creds credentialFlags

	cmd := &cobra.Command{
		Use:  "audit",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if repoDir == "" {
				return errors.New("--repo is required: give the path of the charts repository")
			}
			if err := checkOutputFormat(output); err != nil {
				return err
			}
			cfg, err := rules.Parse(opts.rules)
			if err != nil {
				return err
			}
			given := creds.read(opts)
			opts.log.DebugContext(cmd.Context(), "auditing repository", "repo", repoDir, "output", output)

			report := audit.Run(cmd.Context(), cfg, audit.Options{
				Dir:         repoDir,
				Registry:    registry,
				PlainHTTP:   plainHTTP,
				Credentials: given.credentialFunc(cmd.Context()),
				Fetch:       fetch,
			})
			if err := printAudit(cmd.OutOrStdout(), report, output); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
			if !report.Valid() {
				return errors.New("the repository is not valid")
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&repoDir, "repo", "", "path of the charts repository, which holds the index.yaml")
	flags.StringVar(&registry, "registry", "", "registry path the versions are read from, host[:port]/path, instead of the staging registry of the chart")
	flags.BoolVar(&plainHTTP, "plain-http", false, "talk HTTP to the registry instead of HTTPS, for local registries")
	flags.StringVarP(&output, "output", "o", outputText, "output format: text or json")
	creds.addFlags(flags)
	if err := applyDoc(cmd, "audit"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// printAudit prints the report in the format given.
func printAudit(w io.Writer, report audit.Report, output string) error {
	if output == outputJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(report)
	}
	_, err := io.WriteString(w, auditText(report))
	return err
}

// auditText is the report as plain text: the fields of the JSON, a block for
// each check. An error of more than one line prints one problem to a line.
func auditText(report audit.Report) string {
	blocks := make([]string, 0, len(report))
	for _, r := range report {
		var b strings.Builder
		fmt.Fprintf(&b, "check: %s\nvalid: %t\ndescription: %s\n", r.Check, r.Valid, r.Description)
		if lines := strings.Split(r.Error, "\n"); len(lines) > 1 {
			b.WriteString("error:\n")
			for _, line := range lines {
				b.WriteString("  " + line + "\n")
			}
		} else {
			fmt.Fprintf(&b, "error: %s\n", r.Error)
		}
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}
