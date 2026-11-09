package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nicholasSUSE/cargueiro/internal/chart"
	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// fetchFunc reads Chart.yaml and values.yaml of the chart at ref.
type fetchFunc func(ctx context.Context, ref oci.ChartRef, opts oci.FetchOptions) (*chart.Files, error)

func newCheckOCICmd(opts *rootOptions, fetch fetchFunc) (*cobra.Command, error) {
	var output string
	var noErrorHIP15, plainHTTP bool

	cmd := &cobra.Command{
		Use:  "oci <registry/path/chart:tag>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkOCI(cmd, opts, fetch, args[0], output, noErrorHIP15, plainHTTP)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&output, "output", "o", outputText, "output format: text or json")
	flags.BoolVar(&noErrorHIP15, "no-error-hip-15", false, "do not fail because of helm.sh/images (HIP-15) errors")
	flags.BoolVar(&plainHTTP, "plain-http", false, "talk HTTP to the registry instead of HTTPS, for local registries")
	if err := applyDoc(cmd, "check-oci"); err != nil {
		return nil, err
	}

	return cmd, nil
}

// checkOCI reads the chart at reference from its registry and reports on it.
func checkOCI(cmd *cobra.Command, opts *rootOptions, fetch fetchFunc, reference, output string, noErrorHIP15, plainHTTP bool) error {
	if err := checkOutputFormat(output); err != nil {
		return err
	}
	ref, err := oci.ParseChartRef(reference)
	if err != nil {
		return err
	}
	opts.log.DebugContext(cmd.Context(), "checking chart", "reference", ref.String(), "output", output)

	files, err := fetch(cmd.Context(), ref, oci.FetchOptions{PlainHTTP: plainHTTP})
	if err != nil {
		return err
	}
	return report(cmd, opts, files, output, noErrorHIP15)
}
