package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/rules"
)

// TestEmbeddedRules makes sure the binary carries the rules file at the root of
// the repository, and that it is a valid one.
func TestEmbeddedRules(t *testing.T) {
	onDisk, err := os.ReadFile("rules.yaml")
	require.NoError(t, err)
	require.Equal(t, string(onDisk), string(rulesYAML))

	_, err = rules.Parse(rulesYAML)
	require.NoError(t, err)
}
