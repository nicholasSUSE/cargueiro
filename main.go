// Command cargueiro packages Helm charts and pushes them to OCI registries.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nicholasSUSE/cargueiro/cmd"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root, err := cmd.NewRootCmd(os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	return root.ExecuteContext(ctx)
}
