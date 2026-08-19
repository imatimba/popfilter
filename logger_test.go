package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLevel slog.Level
		wantErr   bool
	}{
		{"debug lower", "debug", slog.LevelDebug, false},
		{"debug upper", "DEBUG", slog.LevelDebug, false},
		{"info mixed", "Info", slog.LevelInfo, false},
		{"WARN upper", "WARN", slog.LevelWarn, false},
		{"warn lower", "warn", slog.LevelWarn, false},
		{"error lower", "error", slog.LevelError, false},
		{"ERROR upper", "ERROR", slog.LevelError, false},
		{"invalid verbose", "verbose", slog.LevelInfo, true},
		{"invalid TRACE", "TRACE", slog.LevelInfo, true},
		{"empty string", "", slog.LevelInfo, true},
		{"info with spaces", " info ", slog.LevelInfo, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := parseLogLevel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseLogLevel(%q) expected error, got nil", tt.input)
				}
				if !strings.Contains(err.Error(), "invalid --log-level") {
					t.Fatalf("error %q should contain 'invalid --log-level'", err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("parseLogLevel(%q) unexpected error: %v", tt.input, err)
				}
				if level != tt.wantLevel {
					t.Fatalf("parseLogLevel(%q) = %v, want %v", tt.input, level, tt.wantLevel)
				}
			}
		})
	}
}

func TestNewLogger_StdErrOnly(t *testing.T) {
	origLogger := slog.Default()
	defer slog.SetDefault(origLogger)

	logger, file, err := NewLogger("info", "")
	if err != nil {
		t.Fatalf("NewLogger unexpected error: %v", err)
	}
	if file != nil {
		t.Fatalf("expected file nil for stderr-only, got %v", file)
	}
	if logger == nil {
		t.Fatalf("expected non-nil logger")
	}
	// Level INFO: debug disabled, info enabled
	if logger.Enabled(nil, slog.LevelDebug) {
		t.Fatalf("INFO logger should not enable DEBUG")
	}
	if !logger.Enabled(nil, slog.LevelInfo) {
		t.Fatalf("INFO logger should enable INFO")
	}
}

func TestNewLogger_HandlerLevelWarn(t *testing.T) {
	logger, file, err := NewLogger("warn", "")
	if err != nil {
		t.Fatalf("NewLogger unexpected error: %v", err)
	}
	if file != nil {
		t.Fatalf("expected file nil")
	}
	if logger.Enabled(nil, slog.LevelInfo) {
		t.Fatalf("WARN logger should not enable INFO")
	}
	if !logger.Enabled(nil, slog.LevelWarn) {
		t.Fatalf("WARN logger should enable WARN")
	}
	if !logger.Enabled(nil, slog.LevelError) {
		t.Fatalf("WARN logger should enable ERROR")
	}
}

func TestNewLogger_DebugEnablesAll(t *testing.T) {
	logger, file, err := NewLogger("debug", "")
	if err != nil {
		t.Fatalf("NewLogger error: %v", err)
	}
	if file != nil {
		t.Fatalf("expected nil file")
	}
	if !logger.Enabled(nil, slog.LevelDebug) {
		t.Fatalf("DEBUG should enable DEBUG")
	}
	if !logger.Enabled(nil, slog.LevelWarn) {
		t.Fatalf("DEBUG should enable WARN")
	}
}

func TestNewLogger_CaseInsensitiveLevel(t *testing.T) {
	logger, file, err := NewLogger("DEBUG", "")
	if err != nil {
		t.Fatalf("NewLogger error: %v", err)
	}
	defer func() { if file != nil { file.Close() } }()
	if !logger.Enabled(nil, slog.LevelDebug) {
		t.Fatalf("DEBUG case-insensitive should enable debug")
	}
}

func TestNewLogger_BufferHandlerLevelSeam(t *testing.T) {
	// Verify Buffer handler seam: create logger with buffer and level filtering
	var buf bytes.Buffer
	// Use factory-like helper via TextHandler directly to verify level semantics
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)
	if logger.Enabled(nil, slog.LevelDebug) {
		t.Fatalf("INFO handler should not enable DEBUG")
	}
	logger.Info("info msg", "key", "value")
	if !strings.Contains(buf.String(), "info msg") {
		t.Fatalf("buffer should contain info msg")
	}
	buf.Reset()
	// Debug suppressed at INFO
	logger.Debug("debug msg")
	if strings.Contains(buf.String(), "debug msg") {
		t.Fatalf("debug msg should be suppressed at INFO level, got %q", buf.String())
	}
}

func TestOpenLogFile_AppendNotTruncate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Seed with prior content
	if err := os.WriteFile(path, []byte("prior\n"), 0644); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	f, err := openLogFile(path)
	if err != nil {
		t.Fatalf("openLogFile error: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString("new\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "prior\n") {
		t.Fatalf("prior content lost (truncated?): %q", content)
	}
	if !strings.Contains(content, "new\n") {
		t.Fatalf("new content missing: %q", content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("mode = %o, want 0644", info.Mode().Perm())
	}
}

func TestOpenLogFile_ParentMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no", "such", "dir", "x.log")
	f, err := openLogFile(path)
	if err == nil {
		if f != nil {
			f.Close()
		}
		t.Fatalf("expected error for missing parent dir, got nil")
	}
	if !strings.Contains(err.Error(), "no such file") {
		// Allow any error, but must be non-nil and parent not created
		t.Logf("got error (acceptable): %v", err)
	}
	// Ensure parent not created via MkdirAll
	if _, statErr := os.Stat(filepath.Dir(path)); statErr == nil {
		t.Fatalf("parent dir should not have been created by openLogFile (MkdirAll forbidden)")
	}
}

func TestOpenLogFile_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "perm.log")
	f, err := openLogFile(path)
	if err != nil {
		t.Fatalf("openLogFile error: %v", err)
	}
	f.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("mode = %o, want 0644", info.Mode().Perm())
	}
}

func TestNewLogger_WithFile_MultiWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.log")
	logger, file, err := NewLogger("info", path)
	if err != nil {
		t.Fatalf("NewLogger error: %v", err)
	}
	if file == nil {
		t.Fatalf("expected file non-nil")
	}
	defer file.Close()

	// Verify file exists and handler writes to both stderr and file
	// Since NewLogger uses io.MultiWriter(os.Stderr, file), file should receive records
	// We can test by logging via the returned logger and reading file
	logger.Info("multi test", "key", "value")
	// Need to sync - close and read
	// File is already open for append; reading after logging should contain message
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "multi test") {
		t.Fatalf("file missing log: %q", string(data))
	}
	if !strings.Contains(string(data), "key=value") && !strings.Contains(string(data), "key=") {
		t.Fatalf("file log missing attrs: %q", string(data))
	}
	// Ensure logger does not contain MkdirAll etc. - checked via static grep in T3
}
