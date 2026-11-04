package logger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		debug     bool
		wantDebug bool
	}{
		{name: "info level hides debug", debug: false, wantDebug: false},
		{name: "debug level shows debug", debug: true, wantDebug: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			New(&buf, tt.debug).Debug("probe")
			require.Equal(t, tt.wantDebug, buf.Len() > 0)
		})
	}
}
