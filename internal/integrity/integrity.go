// Package integrity gives the digest of a file and tells whether two sets of
// digests are the same. It knows nothing about charts or registries: it
// receives file paths or bytes, and names with digests that are already
// calculated.
package integrity

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/opencontainers/go-digest"
)

// File returns the digest of the bytes of the file at path, written
// sha256:<hex>, which is how a registry names the layer of a chart archive.
func File(path string) (digest.Digest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("digest %s: %w", path, err)
	}
	return Bytes(data), nil
}

// Bytes returns the digest of data.
func Bytes(data []byte) digest.Digest {
	return digest.FromBytes(data)
}

// Manifest holds names with their digests. A name is whatever the caller uses
// to tell the entries apart, such as a path inside a chart.
type Manifest struct {
	digests map[string]digest.Digest
}

// NewManifest returns an empty Manifest.
func NewManifest() *Manifest {
	return &Manifest{digests: make(map[string]digest.Digest)}
}

// Add records the digest of name. It fails when name is already there or is
// empty, because two different digests for one name would hide a difference.
func (m *Manifest) Add(name string, d digest.Digest) error {
	if name == "" {
		return fmt.Errorf("an entry of the manifest needs a name")
	}
	if err := d.Validate(); err != nil {
		return fmt.Errorf("digest of %q: %w", name, err)
	}
	if _, found := m.digests[name]; found {
		return fmt.Errorf("%q is in the manifest twice", name)
	}
	m.digests[name] = d
	return nil
}

// Len returns the number of entries.
func (m *Manifest) Len() int {
	if m == nil {
		return 0
	}
	return len(m.digests)
}

// Kind says how an entry differs between two manifests.
type Kind string

// The ways in which two manifests differ.
const (
	// OnlyInA is an entry that the first manifest has and the second does not.
	OnlyInA Kind = "only in the first"
	// OnlyInB is an entry that the second manifest has and the first does not.
	OnlyInB Kind = "only in the second"
	// Changed is an entry both have, with a different digest.
	Changed Kind = "different"
)

// Difference is one entry that two manifests do not agree on. A is its digest
// in the first manifest and B in the second; the one that is not there is empty.
type Difference struct {
	Name string
	Kind Kind
	A, B digest.Digest
}

// Differences is what Compare finds.
type Differences []Difference

// Compare returns the entries that a and b do not agree on, sorted by name. It
// is empty when they are the same. A nil manifest is an empty one.
func Compare(a, b *Manifest) Differences {
	var left, right map[string]digest.Digest
	if a != nil {
		left = a.digests
	}
	if b != nil {
		right = b.digests
	}

	var diffs Differences
	for name, da := range left {
		db, found := right[name]
		switch {
		case !found:
			diffs = append(diffs, Difference{Name: name, Kind: OnlyInA, A: da})
		case da != db:
			diffs = append(diffs, Difference{Name: name, Kind: Changed, A: da, B: db})
		}
	}
	for name, db := range right {
		if _, found := left[name]; !found {
			diffs = append(diffs, Difference{Name: name, Kind: OnlyInB, B: db})
		}
	}
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Name < diffs[j].Name })
	return diffs
}

// Err returns nil when there are no differences. Otherwise it returns an error
// that lists each one, one line per file, calling the first manifest a and the
// second b.
func (ds Differences) Err(a, b string) error {
	if len(ds) == 0 {
		return nil
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "%d files differ between the %s and the %s:", len(ds), a, b)
	for _, d := range ds {
		switch d.Kind {
		case OnlyInA:
			fmt.Fprintf(&msg, "\n  %s: only in the %s", d.Name, a)
		case OnlyInB:
			fmt.Fprintf(&msg, "\n  %s: only in the %s", d.Name, b)
		case Changed:
			fmt.Fprintf(&msg, "\n  %s: the %s has %s and the %s has %s", d.Name, a, d.A, b, d.B)
		}
	}
	return fmt.Errorf("%s", msg.String())
}
