package main

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestPreprocessArgs_TitleWithMediaType(t *testing.T) {
	input := []string{"popfilter", "-title", "The", "Dark", "Knight", "-media-type", "Movies", "-release-group", "BHDStudio", "-video-resolution", "2160p"}
	want := []string{"popfilter", "-title", "The Dark Knight", "-media-type", "Movies", "-release-group", "BHDStudio", "-video-resolution", "2160p"}
	got := preprocessArgs(input)
	if !equalSlices(got, want) {
		t.Fatalf("preprocessArgs(%v) = %v, want %v", input, got, want)
	}
}

func TestPreprocessArgs_TitleWithLogFlags(t *testing.T) {
	input := []string{"popfilter", "-title", "Dune", "--log-level", "debug", "--log-file", "/tmp/x.log", "-media-type", "Movies"}
	want := []string{"popfilter", "-title", "Dune", "--log-level", "debug", "--log-file", "/tmp/x.log", "-media-type", "Movies"}
	got := preprocessArgs(input)
	if !equalSlices(got, want) {
		t.Fatalf("preprocessArgs(%v) = %v, want %v", input, got, want)
	}
	// Title must not absorb --log-level
	if got[2] != "Dune" {
		t.Fatalf("titleParts leaked log flags: got[2]=%q want %q", got[2], "Dune")
	}
}

func TestPreprocessArgs_TitleWithAllFlags(t *testing.T) {
	input := []string{"popfilter", "-title", "The", "Dark", "Knight", "-year", "2008", "-media-type", "Movies", "-release-group", "BHDStudio", "-video-resolution", "2160p", "--tmdb-api-key", "secret123"}
	want := []string{"popfilter", "-title", "The Dark Knight", "-year", "2008", "-media-type", "Movies", "-release-group", "BHDStudio", "-video-resolution", "2160p", "--tmdb-api-key", "secret123"}
	got := preprocessArgs(input)
	if !equalSlices(got, want) {
		t.Fatalf("preprocessArgs(%v) = %v, want %v", input, got, want)
	}
}

func TestPreprocessArgs_NoTitle(t *testing.T) {
	input := []string{"popfilter", "--log-level", "info", "--log-file", "/tmp/a.log"}
	want := []string{"popfilter", "--log-level", "info", "--log-file", "/tmp/a.log"}
	got := preprocessArgs(input)
	if !equalSlices(got, want) {
		t.Fatalf("preprocessArgs(%v) = %v, want %v", input, got, want)
	}
}

func TestPreprocessArgs_TitlePreservation_LogLevelFollowedByTitleWords(t *testing.T) {
	// Verify valueFlags contains --log-level and --log-file
	// This is a direct map check via behavior: title followed by log flags must stop.
	input := []string{"popfilter", "--title", "A", "B", "--log-level", "warn", "--year", "2020"}
	want := []string{"popfilter", "--title", "A B", "--log-level", "warn", "--year", "2020"}
	got := preprocessArgs(input)
	if !equalSlices(got, want) {
		t.Fatalf("preprocessArgs(%v) = %v, want %v", input, got, want)
	}
}

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

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
