package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestScoringObservability requires helper logScoringDecision to exist and emit correct attrs.
func TestScoringObservability_InfoHasDecisionAndCorrelation(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelInfo)
	defer restore()

	cfg := configs["BHDStudio-Movies-2160p"]
	mediaDetails := MediaResult{
		ID:               123,
		Title:            "Dune",
		Popularity:       100.0,
		VoteAverage:      7.5,
		VoteCount:        5000,
		ReleaseYear:      2021,
		OriginalLanguage: "en",
		Genres:           []Genre{{Name: "History"}},
	}
	genres := []string{"History"}
	score := 2.8 // >= 2.5207 => keep
	// This function does not exist yet -> RED
	logScoringDecision(cfg, mediaDetails, "Dune", "Movies", 2021, "BHDStudio", "2160p", "env", true, score, genres)

	logged := buf.String()
	if !strings.Contains(logged, "score decision") {
		t.Fatalf("expected 'score decision' msg, got %q", logged)
	}
	if !strings.Contains(logged, "score=") {
		t.Fatalf("expected score attr, got %q", logged)
	}
	if !strings.Contains(logged, "threshold=") {
		t.Fatalf("expected threshold attr, got %q", logged)
	}
	if !strings.Contains(logged, "decision=keep") {
		t.Fatalf("expected decision=keep, got %q", logged)
	}
	if !strings.Contains(logged, "title=Dune") {
		t.Fatalf("expected title correlation, got %q", logged)
	}
	if !strings.Contains(logged, "media_type=Movies") {
		t.Fatalf("expected media_type, got %q", logged)
	}
	if !strings.Contains(logged, "release_group=BHDStudio") {
		t.Fatalf("expected release_group, got %q", logged)
	}
	if !strings.Contains(logged, "video_resolution=2160p") {
		t.Fatalf("expected video_resolution, got %q", logged)
	}
	if !strings.Contains(logged, "tmdb_id=123") {
		t.Fatalf("expected tmdb_id, got %q", logged)
	}
	if !strings.Contains(logged, "resolved_title=Dune") {
		t.Fatalf("expected resolved_title, got %q", logged)
	}
	if !strings.Contains(logged, "key_source=env") {
		t.Fatalf("expected key_source, got %q", logged)
	}
	if !strings.Contains(logged, "key_present=true") {
		t.Fatalf("expected key_present, got %q", logged)
	}
	// At info level, per-field norms must be suppressed (popularity etc. only in debug)
	if strings.Contains(logged, "popularity=") {
		t.Fatalf("popularity should be absent at INFO level, got %q", logged)
	}
	if strings.Contains(logged, "vote_average=") {
		t.Fatalf("vote_average should be absent at INFO level, got %q", logged)
	}
	// No raw key leak
	if strings.Contains(logged, "secret123") {
		t.Fatalf("log leaked raw key: %q", logged)
	}
}

func TestScoringObservability_DebugHasPerFieldNorms(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	cfg := configs["FLUX-TV-2160p"] // threshold 0.9165
	mediaDetails := MediaResult{
		ID:               456,
		Title:            "Test Show",
		Popularity:       50.0,
		VoteAverage:      8.0,
		VoteCount:        1000,
		ReleaseYear:      2022,
		OriginalLanguage: "en",
		Genres:           []Genre{{Name: "Horror"}, {Name: "Family"}},
	}
	genres := []string{"Horror", "Family"}
	score := 0.5 // < 0.9165 => reject for FLUX-TV-2160p
	logScoringDecision(cfg, mediaDetails, "Test Show", "TV", 2022, "FLUX", "2160p", "flag", true, score, genres)

	logged := buf.String()
	if !strings.Contains(logged, "decision=reject") {
		t.Fatalf("expected decision=reject, got %q", logged)
	}
	if !strings.Contains(logged, "threshold=0.9165") {
		t.Fatalf("expected per-group threshold 0.9165 for FLUX-TV-2160p, got %q", logged)
	}
	// Debug record must contain per-field norms
	if !strings.Contains(logged, "popularity=") {
		t.Fatalf("expected popularity at DEBUG, got %q", logged)
	}
	if !strings.Contains(logged, "vote_average=") {
		t.Fatalf("expected vote_average at DEBUG, got %q", logged)
	}
	if !strings.Contains(logged, "vote_count=") {
		t.Fatalf("expected vote_count at DEBUG, got %q", logged)
	}
	if !strings.Contains(logged, "release_year=") {
		t.Fatalf("expected release_year at DEBUG, got %q", logged)
	}
	if !strings.Contains(logged, "original_language=") {
		t.Fatalf("expected original_language at DEBUG, got %q", logged)
	}
	if !strings.Contains(logged, "genres=") {
		t.Fatalf("expected genres at DEBUG, got %q", logged)
	}
	if !strings.Contains(logged, "weights=") {
		t.Fatalf("expected weights at DEBUG, got %q", logged)
	}
	// correlation still present (TextHandler quotes strings with spaces)
	if !strings.Contains(logged, "resolved_title=") || !strings.Contains(logged, "Test Show") {
		t.Fatalf("expected resolved_title with Test Show, got %q", logged)
	}
}

func TestScoringObservability_NoRawKeyLeak(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	cfg := configs["BHDStudio-Movies-2160p"]
	mediaDetails := MediaResult{ID: 1, Title: "Dune", Popularity: 10, VoteAverage: 5, VoteCount: 100, ReleaseYear: 2020, OriginalLanguage: "en"}
	genres := []string{}
	// Use a secret-like keySource value but ensure raw key string never appears
	secret := "secret123"
	// We pass keySource=flag (redacted), but ensure if someone accidentally logged raw key it would appear
	// Here we verify our helper never logs raw secret even though we have it in scope
	_ = secret
	logScoringDecision(cfg, mediaDetails, "Dune", "Movies", 2021, "BHDStudio", "2160p", "flag", true, 1.0, genres)
	logged := buf.String()
	if strings.Contains(logged, secret) {
		t.Fatalf("log leaked raw key %q in %q", secret, logged)
	}
	if strings.Contains(logged, "Bearer") {
		t.Fatalf("log leaked Bearer in %q", logged)
	}
	// Must have redacted fields
	if !strings.Contains(logged, "key_source=flag") {
		t.Fatalf("expected key_source=flag, got %q", logged)
	}
	if !strings.Contains(logged, "key_present=true") {
		t.Fatalf("expected key_present=true, got %q", logged)
	}
}

func TestScoringObservability_PerGroupThresholdDiffers(t *testing.T) {
	// Verify that same score yields different decisions for different per-group thresholds
	buf, restore := setupBufferLogger(slog.LevelInfo)
	defer restore()

	cfg2160 := configs["FLUX-TV-2160p"] // 0.9165
	cfg1080 := configs["FLUX-TV-1080p"] // 2.5803
	mediaDetails := MediaResult{ID: 1, Title: "Show", Popularity: 10, VoteAverage: 5, VoteCount: 100, ReleaseYear: 2020, OriginalLanguage: "en"}
	genres := []string{}
	score := 1.5 // keep for 2160p, reject for 1080p

	logScoringDecision(cfg2160, mediaDetails, "Show", "TV", 2020, "FLUX", "2160p", "env", true, score, genres)
	if !strings.Contains(buf.String(), "decision=keep") {
		t.Fatalf("FLUX-TV-2160p threshold 0.9165 with score 1.5 should be keep, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "threshold=0.9165") {
		t.Fatalf("expected threshold 0.9165, got %q", buf.String())
	}
	buf.Reset()

	logScoringDecision(cfg1080, mediaDetails, "Show", "TV", 2020, "FLUX", "1080p", "env", true, score, genres)
	if !strings.Contains(buf.String(), "decision=reject") {
		t.Fatalf("FLUX-TV-1080p threshold 2.5803 with score 1.5 should be reject, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "threshold=2.5803") {
		t.Fatalf("expected threshold 2.5803, got %q", buf.String())
	}
}

// Helper to silence unused import if needed
var _ = bytes.Contains
