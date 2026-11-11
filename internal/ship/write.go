package ship

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rancher/cargueiro/internal/fsutil"
)

// nameForm is what the name of a chart and a version may be made of when they
// become the name of a file: nothing that could leave the folder they go in.
var nameForm = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// Shipment is what ship puts in a charts repository: the entry of a version and,
// when there is one to add, the logo of the chart.
type Shipment struct {
	Chart   string
	Version string
	// Entry is the text of the entry.
	Entry []byte
	// Logo is the logo to add. It is nil when the chart needs none, or when the
	// repository has its logo already.
	Logo *Logo
}

// Write writes the entry to entries/<chart>/<version>.yaml and the logo to
// assets/logos/<chart>.<extension> in the charts repository in dir, making the
// folders it needs. Both are written or neither is.
//
// It never replaces anything: it fails, and writes nothing, when the entry is
// there already, even if it would be the same, or when the chart has a logo
// already, whatever its extension.
func (s Shipment) Write(dir string) error {
	return WriteAll(dir, []Shipment{s})
}

// WriteAll writes every shipment as Write does, and all of them or none: nothing
// is written unless every shipment can be, and a failure while writing takes
// back everything written so far.
func WriteAll(dir string, shipments []Shipment) error {
	for _, s := range shipments {
		if err := s.check(dir); err != nil {
			return err
		}
	}

	var batch fsutil.Batch
	for _, s := range shipments {
		if err := s.writeTo(&batch, dir); err != nil {
			return errors.Join(err, batch.Undo())
		}
	}
	return nil
}

// check fails when the shipment cannot be written to the charts repository in
// dir, and writes nothing.
func (s Shipment) check(dir string) error {
	for _, name := range []struct{ what, value string }{{"chart name", s.Chart}, {"version", s.Version}} {
		if !nameForm.MatchString(name.value) {
			return fmt.Errorf("the %s %q is not valid", name.what, name.value)
		}
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder", dir)
	}

	entry := s.entryPath()
	if _, err := os.Lstat(filepath.Join(dir, entry)); err == nil {
		return fmt.Errorf("%s already exists: an entry is never replaced", entry)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if s.Logo != nil {
		existing, err := findLogo(dir, s.Chart)
		if err != nil {
			return err
		}
		if existing != "" {
			return fmt.Errorf("%s is the logo of %s already", existing, s.Chart)
		}
	}
	return nil
}

// writeTo writes the entry and the logo through batch.
func (s Shipment) writeTo(batch *fsutil.Batch, dir string) error {
	if err := batch.Write(filepath.Join(dir, s.entryPath()), s.Entry); err != nil {
		return err
	}
	if s.Logo != nil {
		return batch.Write(filepath.Join(dir, s.logoPath()), s.Logo.Data)
	}
	return nil
}

// Paths returns the files the shipment writes, relative to the charts
// repository: the entry, then the logo if there is one.
func (s Shipment) Paths() []string {
	paths := []string{s.entryPath()}
	if s.Logo != nil {
		paths = append(paths, s.logoPath())
	}
	return paths
}

func (s Shipment) entryPath() string {
	return path.Join(EntriesDir, s.Chart, s.Version+".yaml")
}

func (s Shipment) logoPath() string {
	return path.Join(LogosDir, s.Chart+s.Logo.Extension)
}

// The folders of a charts repository that ship writes to.
const (
	EntriesDir = "entries"
	LogosDir   = "assets/logos"
)

// HasLogo reports whether the charts repository in dir has a logo for the chart,
// whatever its extension.
func HasLogo(dir, chart string) (bool, error) {
	existing, err := findLogo(dir, chart)
	return existing != "", err
}

// findLogo returns the path of the logo of the chart, relative to dir, or "" if
// it has none.
func findLogo(dir, chart string) (string, error) {
	files, err := os.ReadDir(filepath.Join(dir, LogosDir))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", LogosDir, err)
	}
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), chart+".") {
			return path.Join(LogosDir, f.Name()), nil
		}
	}
	return "", nil
}
