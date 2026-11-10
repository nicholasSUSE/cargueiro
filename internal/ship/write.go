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

	"github.com/nicholasSUSE/cargueiro/internal/fsutil"
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
	for _, name := range []struct{ what, value string }{{"chart name", s.Chart}, {"version", s.Version}} {
		if !nameForm.MatchString(name.value) {
			return fmt.Errorf("the %s %q is not valid", name.what, name.value)
		}
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder", dir)
	}

	entry := path.Join(EntriesDir, s.Chart, s.Version+".yaml")
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

	var batch fsutil.Batch
	if err := batch.Write(filepath.Join(dir, entry), s.Entry); err != nil {
		return errors.Join(err, batch.Undo())
	}
	if s.Logo != nil {
		logo := filepath.Join(dir, LogosDir, s.Chart+s.Logo.Extension)
		if err := batch.Write(logo, s.Logo.Data); err != nil {
			return errors.Join(err, batch.Undo())
		}
	}
	return nil
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
