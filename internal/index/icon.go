package index

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	repo "helm.sh/helm/v4/pkg/repo/v1"

	"github.com/nicholasSUSE/cargueiro/internal/rules"
)

// iconPrefix starts the icon of an entry whose logo is kept in the repository.
const iconPrefix = "file://" + AssetsDir + "/" + logosDir + "/"

// checkIcon fails unless the icon of the entry is a logo kept in the charts
// repository in dir: file://assets/logos/<file>, where the file exists. A chart
// that is exempt from having its icon made local is not checked.
func checkIcon(dir string, rule rules.Chart, entry *repo.ChartVersion) error {
	if rule.IconExempt() {
		return nil
	}

	name, found := strings.CutPrefix(entry.Icon, iconPrefix)
	if !found || name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
		return fmt.Errorf("%s %s has the icon %q, but it must be %s<file>", entry.Name, entry.Version, entry.Icon, iconPrefix)
	}

	info, err := os.Lstat(filepath.Join(dir, AssetsDir, logosDir, name))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s %s has the icon %q, but %s/%s/%s does not exist", entry.Name, entry.Version, entry.Icon, AssetsDir, logosDir, name)
	case err != nil:
		return fmt.Errorf("%s %s: check the icon: %w", entry.Name, entry.Version, err)
	case !info.Mode().IsRegular():
		return fmt.Errorf("%s %s has the icon %q, but %s/%s/%s is not a regular file", entry.Name, entry.Version, entry.Icon, AssetsDir, logosDir, name)
	}
	return nil
}

// checkIcons checks the icon of each of the added versions, naming every one
// that is wrong. Versions already published are not checked: they cannot change.
func checkIcons(cfg *rules.Config, dir string, all *repo.IndexFile, added []Ref) error {
	byRef := versionsByRef(all)

	var problems []error
	for _, ref := range added {
		rule, err := cfg.Find(ref.Chart)
		if err != nil {
			return err
		}
		if err := checkIcon(dir, rule, byRef[ref]); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}
