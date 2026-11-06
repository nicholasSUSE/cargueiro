package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootCmd(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "version", args: []string{"version"}, want: "dev (commit none)\n"},
		{name: "version with debug", args: []string{"--debug", "version"}, want: "dev (commit none)\n"},
		{name: "version rejects args", args: []string{"version", "extra"}, wantErr: true},
		{name: "unknown command", args: []string{"nope"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			root, err := NewRootCmd(&stdout, &stderr, nil)
			require.NoError(t, err)
			root.SetArgs(tt.args)

			err = root.Execute()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, stdout.String())
		})
	}
}
