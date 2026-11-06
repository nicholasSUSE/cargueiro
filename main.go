// Command cargueiro packages Helm charts and pushes them to OCI registries.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nicholasSUSE/cargueiro/cmd"
)

// rulesYAML is rules.yaml, compiled into the binary.
//
//go:embed rules.yaml
var rulesYAML []byte

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root, err := cmd.NewRootCmd(os.Stdout, os.Stderr, rulesYAML)
	if err != nil {
		return err
	}

	return root.ExecuteContext(ctx)
}
