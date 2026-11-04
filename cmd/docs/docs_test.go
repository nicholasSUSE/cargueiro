package docs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		want        Doc
		wantMalform string
	}{
		{
			name: "short only",
			src:  "# cmd\n\nDo a thing.\n",
			want: Doc{Short: "Do a thing.", Long: "Do a thing."},
		},
		{
			name: "short and long",
			src:  "# cmd\n\nDo a thing.\n\nFirst detail.\n\nSecond detail.\n",
			want: Doc{Short: "Do a thing.", Long: "Do a thing.\n\nFirst detail.\n\nSecond detail."},
		},
		{
			name: "short wrapped over lines is joined",
			src:  "# cmd\n\nDo a\nthing.\n",
			want: Doc{Short: "Do a thing.", Long: "Do a thing."},
		},
		{
			name: "examples fences stripped, blocks separated by blank line",
			src:  "# cmd\n\nDo a thing.\n\n## Examples\n\n```\ncmd one\n```\n\n```sh\ncmd two\n```\n",
			want: Doc{Short: "Do a thing.", Long: "Do a thing.", Example: "cmd one\n\ncmd two"},
		},
		{
			name: "text outside fences in examples is ignored",
			src:  "# cmd\n\nDo a thing.\n\n## Examples\n\nIntro text.\n\n```\ncmd\n```\n",
			want: Doc{Short: "Do a thing.", Long: "Do a thing.", Example: "cmd"},
		},
		{
			name:        "wrong title",
			src:         "# other\n\nDo a thing.\n",
			wantMalform: "must start with",
		},
		{
			name:        "missing title",
			src:         "Do a thing.\n",
			wantMalform: "must start with",
		},
		{
			name:        "no short description",
			src:         "# cmd\n\n## Examples\n\n```\ncmd\n```\n",
			wantMalform: "no short description",
		},
		{
			name:        "unclosed fence",
			src:         "# cmd\n\nDo a thing.\n\n## Examples\n\n```\ncmd\n",
			wantMalform: "unclosed code fence",
		},
		{
			name:        "examples without code block",
			src:         "# cmd\n\nDo a thing.\n\n## Examples\n\nnothing here\n",
			wantMalform: "no code block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse("cmd", tt.src)

			if tt.wantMalform != "" {
				require.ErrorIs(t, err, ErrMalformed)
				require.ErrorContains(t, err, tt.wantMalform)
				return
			}
			require.NoError(t, err)

			tt.want.Source = tt.src
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("embedded docs load", func(t *testing.T) {
		for _, name := range []string{"package", "version"} {
			doc, err := Load(name)
			require.NoError(t, err, name)
			require.NotEmpty(t, doc.Short, name)
			require.NotEmpty(t, doc.Example, name)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		_, err := Load("nope")
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func TestNames(t *testing.T) {
	names, err := Names()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"package", "version"}, names)
}
