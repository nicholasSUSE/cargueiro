// Command gencerts writes the throwaway TLS material for the e2e registry.
package main

import (
	"fmt"
	"os"

	"github.com/nicholasSUSE/cargueiro/tests/internal/certs"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gencerts <output-dir>")
		os.Exit(2)
	}

	if err := certs.Generate(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "gencerts:", err)
		os.Exit(1)
	}
}
