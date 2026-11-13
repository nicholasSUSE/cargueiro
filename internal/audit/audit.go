// Package audit checks that a charts repository agrees with itself and with the
// registries its versions are pulled from. It reads the repository once, asks
// the registry once for each version, and then compares what it read, so each
// check is a function of data in memory. None of the checks stops the others,
// and the result is a report with one line for each of them.
package audit

import (
	"context"
	"errors"
	"fmt"
	"sort"

	repo "helm.sh/helm/v4/pkg/repo/v1"

	"github.com/rancher/cargueiro/internal/index"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
)

// NoError is the error of a check that passed.
const NoError = "no-error"

// Result is what one check found.
type Result struct {
	// Check is the name of the check.
	Check string `json:"check"`
	// Valid says whether the check passed.
	Valid bool `json:"valid"`
	// Description says in one sentence what the check looked at.
	Description string `json:"description"`
	// Error is NoError, or what the check found wrong, one problem to a line.
	Error string `json:"error"`
}

// Report has the result of every check, in the order they ran.
type Report []Result

// Valid reports whether every check passed.
func (r Report) Valid() bool {
	for _, result := range r {
		if !result.Valid {
			return false
		}
	}
	return true
}

// Options say what to audit.
type Options struct {
	// Dir is the charts repository.
	Dir string
	// Registry replaces the staging registry of the edition of each chart, as
	// host[:port]/path. The name of the chart is added to it.
	Registry string
	// PlainHTTP talks HTTP to the registry instead of HTTPS.
	PlainHTTP bool
	// Credentials gives the credentials to read each registry. Nil reads
	// anonymously.
	Credentials oci.CredentialFunc
	// Fetch reads a manifest from the registry. It defaults to
	// oci.FetchManifestInfo, and tests replace it.
	Fetch FetchFunc
}

// published is a version in the index.yaml of the repository.
type published struct {
	chart string
	entry *repo.ChartVersion
}

// snapshot is everything the checks compare: it is read before the first check
// runs, and no check reads anything else.
type snapshot struct {
	cfg      *rules.Config
	registry string
	// versions are the versions of the index, sorted by chart and version.
	versions []published
	// remote is what the registry answered for each version, by versionName. It
	// is empty until the registry is read.
	remote map[string]remoteVersion
}

// check is one check of the report.
type check struct {
	name string
	// run returns what the check looked at, in one sentence, and what it found
	// wrong, which is nil when the repository passes.
	run func(*snapshot) (string, error)
}

// localChecks are the checks that need nothing but the repository, in the order they run.
func localChecks() []check {
	return []check{
		{name: "urls", run: checkURLs},
	}
}

// remoteChecks are the checks that compare the repository with what the
// registry holds, in the order they run.
func remoteChecks() []check {
	return []check{
		{name: "manifest", run: checkManifests},
	}
}

// Run audits the repository. It reads the repository once, runs the checks that
// need nothing else, reads the registry once for each version, and runs the
// checks that compare the two. A check that cannot run because the repository
// could not be read is in the report as not valid, with the reason, and the
// others still run.
func Run(ctx context.Context, cfg *rules.Config, opts Options) Report {
	snap, readErr := read(cfg, opts)

	var report Report
	for _, c := range localChecks() {
		report = append(report, runCheck(c, snap, readErr))
	}
	if readErr == nil {
		gather(ctx, snap, opts)
	}
	for _, c := range remoteChecks() {
		report = append(report, runCheck(c, snap, readErr))
	}
	return report
}

// read reads the index.yaml of the repository.
func read(cfg *rules.Config, opts Options) (*snapshot, error) {
	idx, err := index.ReadPublished(opts.Dir)
	if err != nil {
		return nil, err
	}

	snap := &snapshot{cfg: cfg, registry: opts.Registry}
	if idx == nil {
		return snap, nil
	}
	for name, versions := range idx.Entries {
		for _, v := range versions {
			snap.versions = append(snap.versions, published{chart: name, entry: v})
		}
	}
	sort.Slice(snap.versions, func(i, j int) bool {
		a, b := snap.versions[i], snap.versions[j]
		if a.chart != b.chart {
			return a.chart < b.chart
		}
		return a.entry.Version < b.entry.Version
	})
	return snap, nil
}

// runCheck runs one check, or says why it could not run.
func runCheck(c check, snap *snapshot, readErr error) Result {
	if readErr != nil {
		return Result{
			Check:       c.name,
			Description: "The check did not run because the repository could not be read.",
			Error:       readErr.Error(),
		}
	}

	description, err := c.run(snap)
	if err != nil {
		return Result{Check: c.name, Description: description, Error: err.Error()}
	}
	return Result{Check: c.name, Valid: true, Description: description, Error: NoError}
}

// problems joins what a check found wrong, or returns nil when it found nothing.
func problems(found []error) error {
	return errors.Join(found...)
}

// versionName names a version for an error.
func versionName(v published) string {
	return fmt.Sprintf("%s %s", v.chart, v.entry.Version)
}
