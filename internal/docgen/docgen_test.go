package docgen

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/cmd"
)

func TestTable(t *testing.T) {
	root := &cobra.Command{Use: "tool"}
	root.AddCommand(
		&cobra.Command{Use: "zeta <arg>", Short: "Last one"},
		&cobra.Command{Use: "alpha", Short: "Has a | pipe"},
		&cobra.Command{Use: "secret", Short: "Hidden", Hidden: true},
		&cobra.Command{Use: "help", Short: "cobra builtin"},
		&cobra.Command{Use: "completion", Short: "cobra builtin"},
	)

	want := "" +
		"| Command | Description |\n" +
		"|---------|-------------|\n" +
		"| [`alpha`](cmd/docs/alpha.md) | Has a \\| pipe |\n" +
		"| [`zeta`](cmd/docs/zeta.md) | Last one |\n"

	require.Equal(t, want, Table(root))
}

func TestReplace(t *testing.T) {
	const table = "| a | b |\n"

	tests := []struct {
		name    string
		readme  string
		want    string
		wantErr string
	}{
		{
			name:   "replaces content between markers",
			readme: "intro\n<!-- COMMANDS:START -->\nold\nstuff\n<!-- COMMANDS:END -->\noutro\n",
			want:   "intro\n<!-- COMMANDS:START -->\n| a | b |\n<!-- COMMANDS:END -->\noutro\n",
		},
		{
			name:   "empty block gets filled",
			readme: "<!-- COMMANDS:START -->\n<!-- COMMANDS:END -->\n",
			want:   "<!-- COMMANDS:START -->\n| a | b |\n<!-- COMMANDS:END -->\n",
		},
		{
			name:    "no markers",
			readme:  "just text\n",
			wantErr: "exactly one",
		},
		{
			name:    "missing end marker",
			readme:  "<!-- COMMANDS:START -->\nold\n",
			wantErr: "exactly one",
		},
		{
			name:    "duplicate start marker",
			readme:  "<!-- COMMANDS:START -->\n<!-- COMMANDS:START -->\n<!-- COMMANDS:END -->\n",
			wantErr: "exactly one",
		},
		{
			name:    "end before start",
			readme:  "<!-- COMMANDS:END -->\n<!-- COMMANDS:START -->\n",
			wantErr: "comes before",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Replace(tt.readme, table)

			if tt.wantErr != "" {
				require.ErrorIs(t, err, ErrMarkers)
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)

			again, err := Replace(got, table)
			require.NoError(t, err)
			require.Equal(t, got, again, "Replace must be idempotent")
		})
	}
}

// TestReadmeIsUpToDate fails when the README commands table no longer matches
// the command docs. Fix it with `make docs`.
func TestReadmeIsUpToDate(t *testing.T) {
	root, err := cmd.NewRootCmd(&bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)

	readme, err := os.ReadFile("../../README.md")
	require.NoError(t, err)

	want, err := Update(string(readme), root)
	require.NoError(t, err)

	require.Equal(t, want, string(readme), "README commands table is stale, run `make docs`")
}
