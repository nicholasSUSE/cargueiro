// Package logger builds the structured loggers used across cargueiro.
package logger

import (
	"io"
	"log/slog"
)

// New returns a text logger writing to w. Debug enables debug-level records.
func New(w io.Writer, debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
