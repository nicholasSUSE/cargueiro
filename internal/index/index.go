// Package index builds the index.yaml of a charts repository from its entries.
//
// An entry is the file entries/<chart>/<version>.yaml: the metadata of one chart
// version and the URL it is pulled from, as `ship` leaves it. The index is only
// a function of the entries, so it is the same bytes every time: the entries
// are put in a fixed order and no time of day is written.
package index

import (
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

// Write builds the index of the charts repository in dir and writes it to
// dir/index.yaml. Nothing is written unless every entry is valid and the entries,
// the index that is already there and assets/ agree.
func Write(cfg *rules.Config, dir string) error {
	idx, err := Build(cfg, dir)
	if err != nil {
		return err
	}

	state, err := readState(dir, idx)
	if err != nil {
		return err
	}
	// The versions to add are used once their archives are pulled; for now it
	// is only checked that the entries, the index and assets/ agree.
	if _, err := state.Added(); err != nil {
		return err
	}

	return idx.WriteFile(filepath.Join(dir, File), 0o600)
}

// Build reads the entries of the charts repository in dir and returns its
// index, with the versions of each chart newest first.
func Build(cfg *rules.Config, dir string) (*repo.IndexFile, error) {
	versions := cfg.Versions()
	policy, err := version.NewPolicy(versions.Prereleases, versions.Majors)
	if err != nil {
		return nil, err
	}

	charts, err := os.ReadDir(filepath.Join(dir, EntriesDir))
	if err != nil {
		return nil, fmt.Errorf("read entries: %w", err)
	}

	// The time of day is left out: the zero time is what Rancher writes too.
	idx := &repo.IndexFile{APIVersion: "v1", Entries: map[string]repo.ChartVersions{}}
	for _, c := range charts {
		if !c.IsDir() {
			return nil, fmt.Errorf("%s/%s is not a chart directory", EntriesDir, c.Name())
		}
		entries, err := readChart(cfg, policy, filepath.Join(dir, EntriesDir, c.Name()), c.Name())
		if err != nil {
			return nil, err
		}
		idx.Entries[c.Name()] = entries
	}
	return idx, nil
}

// readChart reads the entries of one chart and returns them newest first.
func readChart(cfg *rules.Config, policy *version.Policy, dir, name string) (repo.ChartVersions, error) {
	rule, err := cfg.Find(name)
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read entries of %s: %w", name, err)
	}

	byVersion := map[string]*repo.ChartVersion{}
	var problems []error
	for _, f := range files {
		entry, err := readEntry(filepath.Join(dir, f.Name()), name, f.Name())
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if _, found := byVersion[entry.Version]; found {
			problems = append(problems, fmt.Errorf("%s version %s is in more than one entry", name, entry.Version))
			continue
		}
		byVersion[entry.Version] = entry
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	ordered, err := policy.SortNewestFirst(slices.Collect(maps.Keys(byVersion)), rule.PlainVersion)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	entries := make(repo.ChartVersions, 0, len(ordered))
	for _, v := range ordered {
		entries = append(entries, byVersion[v])
	}
	return entries, nil
}

// readEntry reads the entry in path, which must be the file of chart name.
func readEntry(path, name, file string) (*repo.ChartVersion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read entry: %w", err)
	}
	var entry repo.ChartVersion
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
