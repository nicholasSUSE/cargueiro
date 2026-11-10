// Package index builds the index.yaml of a charts repository from its entries.
//
// An entry is the file entries/<chart>/<version>.yaml: the metadata of one chart
// version and the URL it is pulled from, as `ship` leaves it. The index is only
// a function of the entries and the archives in assets/, so it is the same bytes
// every time: the entries are put in a fixed order and no time of day is written.
package index

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	repo "helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"

	"github.com/nicholasSUSE/cargueiro/internal/rules"
	"github.com/nicholasSUSE/cargueiro/internal/version"
)

const (
	// EntriesDir is where the entries are, inside a charts repository.
	EntriesDir = "entries"
	// File is the name of the index inside a charts repository.
	File = "index.yaml"
)

// Options say how to write the index.
type Options struct {
	// Registry replaces the staging registry of the chart's edition that the
	// URLs of new versions must point to, as host[:port]/path.
	Registry string
	// PlainHTTP talks HTTP to the registry the archives are pulled from.
	PlainHTTP bool
	// Pull reads an archive from the registry. It defaults to oci.PullArchive.
	Pull PullFunc
	// DryRun makes Write check, and report what it would do, without pulling
	// or writing anything.
	DryRun bool
}

// Write builds the index of the charts repository in dir and writes it to
// dir/index.yaml. Hidden versions are not written to it. It returns what it did:
// the versions it added to the index and the ones it left out of it.
//
// Nothing is written unless every entry is valid, the entries, the index that is
// already there and assets/ agree, no published version or archive has changed,
// the URLs and icons of the new versions are right, and the archive of every new
// version has been pulled and is the chart its entry describes. The archives are
// written to assets/ before the index is, and are taken back if the index cannot
// be written.
//
// With Options.DryRun it does every check that needs no registry, pulls and
// writes nothing, and returns what it would do.
func Write(ctx context.Context, cfg *rules.Config, dir string, opts Options) (Plan, error) {
	entries, err := readEntries(cfg, dir)
	if err != nil {
		return Plan{}, err
	}
	published, err := readPublished(dir)
	if err != nil {
		return Plan{}, err
	}

	state, err := readState(dir, entries, published)
	if err != nil {
		return Plan{}, err
	}
	plan, err := state.Plan()
	if err != nil {
		return Plan{}, err
	}
	digests, err := archiveDigests(dir, entries.all)
	if err != nil {
		return Plan{}, err
	}
	if err := checkArchivesUnchanged(published, digests); err != nil {
		return Plan{}, err
	}
	setDigests(entries.all, digests)
	if err := checkUnchanged(published, entries.all); err != nil {
		return Plan{}, err
	}
	if err := errors.Join(
		checkURLs(cfg, opts.Registry, entries.all, plan.Add),
		checkIcons(cfg, dir, entries.all, plan.Add),
	); err != nil {
		return Plan{}, err
	}
	if opts.DryRun {
		return plan, nil
	}

	archives, err := pullAdded(ctx, entries.all, plan.Add, opts)
	if err != nil {
		return Plan{}, err
	}
	pulled := make(map[Ref]string, len(archives))
	for ref, archive := range archives {
		pulled[ref] = digestOf(archive)
	}
	setDigests(entries.all, pulled)

	undo, err := writeArchives(dir, archives)
	if err != nil {
		return Plan{}, err
	}
	if err := entries.visible().WriteFile(filepath.Join(dir, File), 0o600); err != nil {
		return Plan{}, errors.Join(err, undo())
	}
	return plan, nil
}

// entries are the entries of a charts repository, as an index and as the
// versions among them that are hidden.
type entries struct {
	// all holds every entry, hidden or not.
	all    *repo.IndexFile
	hidden []Ref
}

// visible returns the index to publish: all the entries but the hidden ones.
func (e entries) visible() *repo.IndexFile {
	hidden := refSet(e.hidden)

	idx := &repo.IndexFile{APIVersion: e.all.APIVersion, Entries: map[string]repo.ChartVersions{}}
	for name, versions := range e.all.Entries {
		for _, v := range versions {
			if _, isHidden := hidden[Ref{Chart: name, Version: v.Version}]; !isHidden {
				idx.Entries[name] = append(idx.Entries[name], v)
			}
		}
	}
	return idx
}

// readEntries reads the entries of the charts repository in dir, with the
// versions of each chart newest first.
func readEntries(cfg *rules.Config, dir string) (entries, error) {
	versions := cfg.Versions()
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return entries{}, err
	}

	charts, err := os.ReadDir(filepath.Join(dir, EntriesDir))
	if err != nil {
		return entries{}, fmt.Errorf("read entries: %w", err)
	}

	// The time of day is left out: the zero time is what Rancher writes too.
	read := entries{all: &repo.IndexFile{APIVersion: "v1", Entries: map[string]repo.ChartVersions{}}}
	for _, c := range charts {
		if !c.IsDir() {
			return entries{}, fmt.Errorf("%s/%s is not a chart directory", EntriesDir, c.Name())
		}
		versions, hidden, err := readChart(cfg, policy, filepath.Join(dir, EntriesDir, c.Name()), c.Name())
		if err != nil {
			return entries{}, err
		}
		read.all.Entries[c.Name()] = versions
		read.hidden = append(read.hidden, hidden...)
	}
	return read, nil
}

// readChart reads the entries of one chart and returns them newest first, and
// the versions among them that are hidden.
func readChart(cfg *rules.Config, policy *version.Policy, dir, name string) (repo.ChartVersions, []Ref, error) {
	rule, err := cfg.Find(name)
	if err != nil {
		return nil, nil, err
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read entries of %s: %w", name, err)
	}

	byVersion := map[string]*repo.ChartVersion{}
	var hidden []Ref
	var problems []error
	for _, f := range files {
		file, err := readEntry(filepath.Join(dir, f.Name()), name, f.Name())
		if err != nil {
			problems = append(problems, err)
			continue
		}
		entry := &file.ChartVersion
		if _, found := byVersion[entry.Version]; found {
			problems = append(problems, fmt.Errorf("%s version %s is in more than one entry", name, entry.Version))
			continue
		}
		byVersion[entry.Version] = entry
		if file.Hide {
			hidden = append(hidden, Ref{Chart: name, Version: entry.Version})
		}
	}
	if len(problems) > 0 {
		return nil, nil, errors.Join(problems...)
	}

	ordered, err := policy.SortNewestFirst(slices.Collect(maps.Keys(byVersion)), rule.PlainVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", name, err)
	}
	versions := make(repo.ChartVersions, 0, len(ordered))
	for _, v := range ordered {
		versions = append(versions, byVersion[v])
	}
	return versions, hidden, nil
}

// entryFile is an entry as it is written in entries/: the version as the index
// lists it, and whether the version is hidden.
type entryFile struct {
	repo.ChartVersion
	// Hide takes the version out of the index, without removing the entry or
	// the archive. It is not a field of the index.
	Hide bool `json:"hide,omitempty"`
}

// readEntry reads the entry in path, which must be the file of chart name.
func readEntry(path, name, file string) (*entryFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read entry: %w", err)
	}
	var entry entryFile
	if err := yaml.UnmarshalStrict(data, &entry); err != nil {
		return nil, fmt.Errorf("entry %s/%s: %w", name, file, err)
	}
	if entry.Metadata == nil {
		return nil, fmt.Errorf("entry %s/%s is empty", name, file)
	}
	if entry.Name != name {
		return nil, fmt.Errorf("entry %s/%s is of chart %q", name, file, entry.Name)
	}
	if want := entry.Version + ".yaml"; file != want {
		return nil, fmt.Errorf("entry %s/%s holds version %s, so the file must be called %s", name, file, entry.Version, want)
	}
	entry.Created = time.Time{}
	return &entry, nil
}
