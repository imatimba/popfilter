package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// NewLogger creates a TextHandler logger for the log destination.
// levelStr is case-insensitive debug|info|warn|error; filePath is "" for stderr-only.
// Returns (*slog.Logger, *os.File, error). File is nil when filePath == "".
// Caller (main only) must defer file.Close() when non-nil and set it as default.
// On invalid log level or unopenable file, returns nil, nil, and a wrapped error
// (no process exit) so the caller can decide how to report usage and exit.
func NewLogger(levelStr, filePath string) (*slog.Logger, *os.File, error) {
	if levelStr == "" {
		levelStr = "info"
	}
	level, err := parseLogLevel(levelStr)
	if err != nil {
		return nil, nil, err
	}

	var w io.Writer = os.Stderr
	var f *os.File

	if filePath != "" {
		f, err = openLogFile(filePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open --log-file %q: %w", filePath, err)
		}
		w = io.MultiWriter(os.Stderr, f)
	}

	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	return logger, f, nil
}

// parseLogLevel normalizes and validates. Used only inside NewLogger.
func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid --log-level %q: must be debug|info|warn|error", s)
	}
}

// openLogFile opens with append, create, write-only 0644. Parent must exist, no truncation.
func openLogFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return f, nil
}
