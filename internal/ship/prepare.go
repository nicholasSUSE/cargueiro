package ship

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rancher/cargueiro/internal/chart"
	"github.com/rancher/cargueiro/internal/check"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
	"github.com/rancher/cargueiro/internal/version"
)

// ResolveFunc turns the tag of ref into the digest of its manifest.
type ResolveFunc func(ctx context.Context, ref oci.ChartRef, opts oci.FetchOptions) (string, error)

// PullFunc returns the archive of the manifest that digest pins in repo.
type PullFunc func(ctx context.Context, repo oci.ChartRepo, digest string, opts oci.FetchOptions) ([]byte, error)

// CreatedFunc returns when the chart of the manifest that digest pins in repo was
// pushed.
type CreatedFunc func(ctx context.Context, repo oci.ChartRepo, digest string, opts oci.FetchOptions) (time.Time, error)

// LogoFunc downloads the logo at an address.
type LogoFunc func(ctx context.Context, address string) (Logo, error)

// Options say how to prepare a shipment. The functions default to the registry
// and the web, and are replaced in tests.
type Options struct {
	// PlainHTTP talks HTTP to the registry instead of HTTPS.
	PlainHTTP bool
	// Credentials gives the credentials to read from the registry. Without it
	// the reads are anonymous.
	Credentials oci.CredentialFunc
	Resolve     ResolveFunc
	Pull        PullFunc
	Created     CreatedFunc
	Logo        LogoFunc
}

// InvalidChartError says that the chart does not follow the chart rules. The
// Result has what is wrong with it.
type InvalidChartError struct {
	Result *check.Result
}

func (e *InvalidChartError) Error() string {
	return fmt.Sprintf("%s %s does not follow the chart rules", e.Result.Chart, e.Result.Metadata.Version)
}

// Prepare works out what to ship for the chart at ref to the charts repository in
// dir, and writes nothing. The tag is resolved to the digest of its manifest and
// the archive is read by that digest, so the chart and the digest cannot
// disagree. The chart must follow the chart rules, and its icon is made local:
// the logo is downloaded unless the repository has it, and an exempt chart is
// left as it is.
func Prepare(ctx context.Context, cfg *rules.Config, ref oci.ChartRef, dir string, opts Options) (Shipment, error) {
	resolve, pull, created, fetchLogo := opts.Resolve, opts.Pull, opts.Created, opts.Logo
	if resolve == nil {
		resolve = oci.ResolveDigest
	}
	if pull == nil {
		pull = oci.PullArchive
	}
	if created == nil {
		created = oci.ManifestCreated
	}
	if fetchLogo == nil {
		client := &http.Client{Timeout: 30 * time.Second}
		fetchLogo = func(ctx context.Context, address string) (Logo, error) { return FetchLogo(ctx, client, address) }
	}
	fetch, err := opts.Credentials.FetchOptions(ref.Host, opts.PlainHTTP)
	if err != nil {
		return Shipment{}, err
	}

	digest, err := resolve(ctx, ref, fetch)
	if err != nil {
		return Shipment{}, err
	}
	chartRepo := oci.ChartRepo{Host: ref.Host, Repository: ref.Repository}
	pushed, err := created(ctx, chartRepo, digest, fetch)
	if err != nil {
		return Shipment{}, fmt.Errorf("read when the chart was pushed: %w", err)
	}
	archive, err := pull(ctx, chartRepo, digest, fetch)
	if err != nil {
		return Shipment{}, fmt.Errorf("pull the archive: %w", err)
	}
	files, err := chart.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return Shipment{}, fmt.Errorf("read the chart in %s: %w", ref, err)
	}
	chartYAML, err := chart.ArchiveChartYAML(bytes.NewReader(archive))
	if err != nil {
		return Shipment{}, fmt.Errorf("read the chart in %s: %w", ref, err)
	}

	name, chartVersion := files.Metadata.Name, files.Metadata.Version
	if tagVersion := version.FromOCITag(ref.Tag); tagVersion != chartVersion {
		return Shipment{}, fmt.Errorf("the reference says version %s but the chart is %s", tagVersion, chartVersion)
	}
	if path.Base(ref.Repository) != name {
		return Shipment{}, fmt.Errorf("the repository %q is not named after the chart %q", ref.Repository, name)
	}

	result, err := check.Chart(cfg, files, chartVersion)
	if err != nil {
		return Shipment{}, err
	}
	if !result.Valid {
		return Shipment{}, &InvalidChartError{Result: result}
	}
	rule, err := cfg.Find(name)
	if err != nil {
		return Shipment{}, err
	}

	icon, logo, err := localIcon(ctx, rule, files.Metadata.Icon, dir, fetchLogo)
	if err != nil {
		return Shipment{}, err
	}
	// The tag is written as the registry stores it, with the + of the version as a _.
	url := "oci://" + ref.Host + "/" + ref.Repository + ":" + version.ToOCITag(chartVersion) + "@" + digest
	entry, err := BuildEntry(chartYAML, icon, url, pushed)
	if err != nil {
		return Shipment{}, err
	}
	return Shipment{Chart: name, Version: chartVersion, Entry: entry, Logo: logo}, nil
}

// localIcon works out the icon of the entry of a chart whose icon is icon, and
// the logo to add to the repository in dir for it. An exempt chart keeps its
// icon, and has no logo. For any other chart the icon is the logo the repository
// has, or the one downloaded from the address of icon.
func localIcon(ctx context.Context, rule rules.Chart, icon, dir string, fetchLogo LogoFunc) (string, *Logo, error) {
	if rule.IconExempt() {
		return "", nil, nil
	}
	if icon == "" {
		return "", nil, fmt.Errorf("%s has no icon, and a chart that is not exempt needs one", rule.Name)
	}
	if strings.HasPrefix(icon, "file://") {
		return "", nil, fmt.Errorf("the icon %q of %s is a local file already", icon, rule.Name)
	}

	existing, err := findLogo(dir, rule.Name)
	if err != nil {
		return "", nil, err
	}
	if existing != "" {
		return "file://" + existing, nil, nil
	}

	logo, err := fetchLogo(ctx, icon)
	if err != nil {
		return "", nil, err
	}
	return "file://" + path.Join(LogosDir, rule.Name+logo.Extension), &logo, nil
}
