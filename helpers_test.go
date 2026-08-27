package main

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestGetTMDBAPIKey_RedactedLogging_Env(t *testing.T) {
	origLogger := slog.Default()
	defer slog.SetDefault(origLogger)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	t.Setenv("TMDB_API_KEY", "secret123")
	// Ensure .env does not interfere - unset file? godotenv.Load will run but env already set wins.
	apiKeyArg := "flagKey"
	key, source, err := getTMDBAPIKey(&apiKeyArg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "secret123" {
		t.Fatalf("key = %q, want %q", key, "secret123")
	}
	if source != "env" {
		t.Fatalf("source = %q, want %q", source, "env")
	}
	logged := buf.String()
	if !strings.Contains(logged, "key_source=env") {
		t.Fatalf("log missing key_source=env: %q", logged)
	}
	if !strings.Contains(logged, "key_present=true") {
		t.Fatalf("log missing key_present=true: %q", logged)
	}
	if strings.Contains(logged, "secret123") {
		t.Fatalf("log leaked secret: %q", logged)
	}
}

func TestGetTMDBAPIKey_RedactedLogging_Flag(t *testing.T) {
	origLogger := slog.Default()
	defer slog.SetDefault(origLogger)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	// Clear env
	t.Setenv("TMDB_API_KEY", "")
	if err := os.Unsetenv("TMDB_API_KEY"); err != nil {
		t.Fatalf("Unsetenv: %v", err)
	}
	// Remove .env effect by ensuring no TMDB_API_KEY in file
	// We call godotenv.Load inside function; if .env has key, it would win over flag.
	// Test in temp dir without .env
	origWd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	apiKeyArg := "flagSecret999"
	key, source, err := getTMDBAPIKey(&apiKeyArg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "flagSecret999" {
		t.Fatalf("key = %q, want flagSecret999", key)
	}
	if source != "flag" {
		t.Fatalf("source = %q, want flag", source)
	}
	logged := buf.String()
	if strings.Contains(logged, "flagSecret999") {
		t.Fatalf("log leaked flag secret: %q", logged)
	}
	if !strings.Contains(logged, "key_source=flag") {
		t.Fatalf("log missing key_source=flag: %q", logged)
	}
}

func TestGetTMDBAPIKey_MissingLogsError(t *testing.T) {
	origLogger := slog.Default()
	defer slog.SetDefault(origLogger)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	t.Setenv("TMDB_API_KEY", "")
	if err := os.Unsetenv("TMDB_API_KEY"); err != nil {
		t.Fatalf("Unsetenv: %v", err)
	}
	origWd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	empty := ""
	_, source, err := getTMDBAPIKey(&empty)
	if err == nil {
		t.Fatalf("expected error for missing key")
	}
	if source != "missing" {
		t.Fatalf("source = %q, want missing", source)
	}
	logged := buf.String()
	if !strings.Contains(logged, "key_source=missing") {
		t.Fatalf("log missing key_source=missing: %q", logged)
	}
}


