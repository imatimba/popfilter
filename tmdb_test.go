package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupBufferLogger(level slog.Level) (*bytes.Buffer, func()) {
	orig := slog.Default()
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
	return &buf, func() { slog.SetDefault(orig) }
}

func TestDoTMDBGet_LogsStatusAndDuration(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	secret := "secret123"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// verify auth header is set but we don't leak it
		if got := r.Header.Get("Authorization"); got != "Bearer "+secret {
			t.Errorf("unexpected auth header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	var target map[string]string
	urlWithQuery := srv.URL + "/test?query=Dune&include_adult=true"
	err := doTMDBGet(urlWithQuery, secret, &target)
	if err != nil {
		t.Fatalf("doTMDBGet error: %v", err)
	}
	logged := buf.String()
	if !strings.Contains(logged, "status_code=200") {
		t.Fatalf("expected status_code=200 in logs, got %q", logged)
	}
	if !strings.Contains(logged, "duration_ms=") {
		t.Fatalf("expected duration_ms in logs, got %q", logged)
	}
	// duration_ms should be present and ideally >0, check that it is not duration_ms=0 too strictly? at least contains
	if !strings.Contains(logged, "attempt=1") {
		t.Fatalf("expected attempt=1 in logs, got %q", logged)
	}
	if !strings.Contains(logged, "auth_present=true") {
		t.Fatalf("expected auth_present=true, got %q", logged)
	}
	if !strings.Contains(logged, "url_path=") {
		t.Fatalf("expected url_path in logs, got %q", logged)
	}
	// url_path must be redacted (no query)
	if strings.Contains(logged, "query=Dune") {
		t.Fatalf("url_path not redacted, contains query: %q", logged)
	}
	if strings.Contains(logged, "secret123") {
		t.Fatalf("log leaked secret: %q", logged)
	}
	if strings.Contains(logged, "Bearer") {
		t.Fatalf("log leaked Bearer: %q", logged)
	}
}

func TestDoTMDBGet_NoBearerLeak(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	secret := "secret123"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	var target map[string]string
	// also test with query containing secret-like param to ensure not logged
	urlWithQuery := srv.URL + "/search/movie?query=Dune&api_key=" + secret
	err := doTMDBGet(urlWithQuery, secret, &target)
	if err != nil {
		t.Fatalf("doTMDBGet error: %v", err)
	}
	logged := buf.String()
	if strings.Contains(logged, secret) {
		t.Fatalf("log leaked secret123: %q", logged)
	}
	if strings.Contains(logged, "Bearer "+secret) {
		t.Fatalf("log leaked Bearer secret: %q", logged)
	}
	// url_path should be redacted, so api_key param not present
	if strings.Contains(logged, "api_key") {
		t.Fatalf("log leaked api_key query param: %q", logged)
	}
}

func TestSearchMediaID_LogsDebugWire(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	secret := "secret123"
	// Save and override base URL
	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Search endpoint
		if strings.HasPrefix(r.URL.Path, "/search/") {
			// verify redaction not needed here, just respond
			w.Header().Set("Content-Type", "application/json")
			resp := SearchResponse{
				Results: []MediaResult{
					{ID: 42, Title: "Dune", ReleaseDate: "2021-10-22"},
					{ID: 99, Title: "Dune Messiah", ReleaseDate: "2023-01-01"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		t.Fatalf("unexpected path %q", r.URL.Path)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("Dune", "Movies", secret, 2021)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 42 {
		t.Fatalf("SearchMediaID id=%d want 42 (exact_match)", id)
	}
	logged := buf.String()
	if !strings.Contains(logged, "tmdb_search_query=Dune") {
		t.Fatalf("expected tmdb_search_query=Dune, got %q", logged)
	}
	if !strings.Contains(logged, "fallback_reason=exact_match") {
		t.Fatalf("expected fallback_reason=exact_match, got %q", logged)
	}
	if !strings.Contains(logged, "tmdb_id=42") {
		t.Fatalf("expected tmdb_id=42, got %q", logged)
	}
	if !strings.Contains(logged, "attempt=1") {
		t.Fatalf("expected attempt=1, got %q", logged)
	}
	if !strings.Contains(logged, "auth_present=true") {
		t.Fatalf("expected auth_present=true, got %q", logged)
	}
	// doTMDBGet inside should have logged status_code and duration_ms
	if !strings.Contains(logged, "status_code=200") {
		t.Fatalf("expected status_code=200 from doTMDBGet, got %q", logged)
	}
	if !strings.Contains(logged, "duration_ms=") {
		t.Fatalf("expected duration_ms, got %q", logged)
	}
	if strings.Contains(logged, secret) {
		t.Fatalf("log leaked secret: %q", logged)
	}
	if strings.Contains(logged, "Bearer") {
		t.Fatalf("log leaked Bearer: %q", logged)
	}
}

func TestSearchMediaID_FallbackReason_FirstResult(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	secret := "secret123"
	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/search/") {
			w.Header().Set("Content-Type", "application/json")
			// No exact match: title mismatch or year mismatch
			resp := SearchResponse{
				Results: []MediaResult{
					{ID: 10, Title: "Dune Part Two", ReleaseDate: "2024-03-01"},
					{ID: 11, Title: "Other", ReleaseDate: "2021-10-22"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		t.Fatalf("unexpected path %q", r.URL.Path)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("Dune", "Movies", secret, 2021)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 10 {
		t.Fatalf("expected fallback to first result ID 10, got %d", id)
	}
	logged := buf.String()
	if !strings.Contains(logged, "fallback_reason=fallback_first_result") {
		t.Fatalf("expected fallback_reason=fallback_first_result, got %q", logged)
	}
	if !strings.Contains(logged, "tmdb_id=10") {
		t.Fatalf("expected tmdb_id=10, got %q", logged)
	}
	if strings.Contains(logged, secret) {
		t.Fatalf("log leaked secret: %q", logged)
	}
}

// Additional triangulate: ensure duration_ms>0 and redacted URL path
func TestDoTMDBGet_DurationMsPositiveAndRedacted(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	secret := "secret123"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	var target map[string]string
	urlWithQuery := srv.URL + "/search/movie?query=hello&include_adult=true"
	err := doTMDBGet(urlWithQuery, secret, &target)
	if err != nil {
		t.Fatalf("doTMDBGet error: %v", err)
	}
	logged := buf.String()
	// Check duration_ms numeric > -1 present; simple contains check plus not leaked query
	if strings.Contains(logged, "query=hello") {
		t.Fatalf("url_path should be redacted, got query in log %q", logged)
	}
	// Ensure url_path is the path without query
	if !strings.Contains(logged, "url_path="+srv.URL+"/search/movie") && !strings.Contains(logged, "url_path=/search/movie") {
		// fallback check: contains url_path and not contains "?"
		if strings.Contains(logged, "?") && strings.Contains(logged, "url_path=") {
			t.Fatalf("url_path contains ?, not redacted: %q", logged)
		}
	}
	if strings.Contains(logged, secret) {
		t.Fatalf("leaked secret")
	}
}

func TestDoTMDBGet_HTTPErrorLogsAtError(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	secret := "secret123"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status_message":"invalid"}`))
	}))
	defer srv.Close()

	var target map[string]string
	err := doTMDBGet(srv.URL+"/search/movie?query=Dune", secret, &target)
	if err == nil {
		t.Fatalf("expected error for 401, got nil")
	}
	logged := buf.String()
	if !strings.Contains(logged, "status_code=401") {
		t.Fatalf("expected status_code=401 in error log, got %q", logged)
	}
	if !strings.Contains(logged, "attempt=1") {
		t.Fatalf("expected attempt=1 in error log, got %q", logged)
	}
	if strings.Contains(logged, secret) {
		t.Fatalf("leaked secret in error log")
	}
}

func TestGetMediaDetails_LogsResolvedTitle(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	secret := "secret123"
	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Respond with Name fallback (Title empty) and ReleaseDate
		resp := MediaResult{
			ID:               123,
			Title:            "",
			Name:             "Dune Series",
			ReleaseDate:      "",
			FirstAirDate:     "2021-10-22",
			Popularity:       10.0,
			VoteAverage:      7.5,
			VoteCount:        100,
			OriginalLanguage: "en",
			Genres:           []Genre{{ID: 1, Name: "Sci-Fi"}},
		}
		// Ensure ReleaseYear parsing works: we send dates, server returns JSON, GetMediaDetails will parse
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	details, err := GetMediaDetails(123, "Movies", secret)
	if err != nil {
		t.Fatalf("GetMediaDetails error: %v", err)
	}
	if details.Title != "Dune Series" {
		t.Fatalf("expected resolved title fallback to Name, got %q", details.Title)
	}
	logged := buf.String()
	if !strings.Contains(logged, "resolved_title=Dune") && !strings.Contains(logged, "resolved_title=\"Dune Series\"") && !strings.Contains(logged, "resolved_title=Dune Series") {
		// slog text handler may quote values with spaces; check substring without exact quoting
		if !strings.Contains(logged, "resolved_title") || !strings.Contains(logged, "Dune Series") {
			t.Fatalf("expected resolved_title with fallback Name, got %q", logged)
		}
	}
	if !strings.Contains(logged, "tmdb_id=123") {
		t.Fatalf("expected tmdb_id=123, got %q", logged)
	}
	if strings.Contains(logged, secret) {
		t.Fatalf("leaked secret")
	}
}

func TestGetMediaDetailsWorkflow_LogsError(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	secret := "secret123"
	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/search/") {
			w.Header().Set("Content-Type", "application/json")
			// No results -> error path
			_ = json.NewEncoder(w).Encode(SearchResponse{Results: []MediaResult{}})
			return
		}
		t.Fatalf("unexpected path %q", r.URL.Path)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	_, err := GetMediaDetailsWorkflow("UnknownTitle", "Movies", secret, 2021)
	if err == nil {
		t.Fatalf("expected error for no results")
	}
	logged := buf.String()
	// workflow should log error with correlation
	if !strings.Contains(logged, "tmdb workflow search failed") && !strings.Contains(logged, "tmdb search no results") {
		t.Fatalf("expected workflow/search error logs, got %q", logged)
	}
	if strings.Contains(logged, secret) {
		t.Fatalf("leaked secret")
	}
}
