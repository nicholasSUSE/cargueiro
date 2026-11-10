// Command docgen rewrites the commands table in README.md from the command docs.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rancher/cargueiro/cmd"
	"github.com/rancher/cargueiro/internal/docgen"
)

func main() {
	if err := run("README.md"); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}

func run(readmePath string) error {
	root, err := cmd.NewRootCmd(io.Discard, io.Discard, nil)
	if err != nil {
		return err
	}

	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", readmePath, err)
	}

	updated, err := docgen.Update(string(readme), root)
	if err != nil {
		return err
	}
	if updated == string(readme) {
		return nil
	}

	if err := os.WriteFile(readmePath, []byte(updated), 0o644); err != nil { //nolint:gosec // README is a public file
		return fmt.Errorf("write %s: %w", readmePath, err)
	}
	return nil
}
