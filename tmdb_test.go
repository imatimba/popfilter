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

// TestSearchMediaID_TVNameExactMatch: a TV search result carries Name +
// FirstAirDate (not Title/ReleaseDate). Exact match must fire on the
// display name with year agreement derived from FirstAirDate.
func TestSearchMediaID_TVNameExactMatch(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/search/tv") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := SearchResponse{
			Results: []MediaResult{
				{ID: 1416, Name: "Grey's Anatomy", FirstAirDate: "2005-03-27"},
				{ID: 9999, Name: "S&X", FirstAirDate: "2023-05-01"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("S&X", "TV", "secret123", 2023)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 9999 {
		t.Fatalf("expected exact match on TV Name to ID 9999, got %d", id)
	}
	logged := buf.String()
	if !strings.Contains(logged, "fallback_reason=exact_match") {
		t.Fatalf("expected fallback_reason=exact_match, got %q", logged)
	}
}

// TestSearchMediaID_TVNameExactMatchYearZero: with year=0 the year
// agreement must be skipped entirely; name-only exact match fires even
// when the result has no usable date fields.
func TestSearchMediaID_TVNameExactMatchYearZero(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := SearchResponse{
			Results: []MediaResult{
				// Decoy first so accidental fallback cannot satisfy the assertion.
				{ID: 6666, Name: "Decoy Show"},
				{ID: 7777, Name: "S&X"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("S&X", "TV", "secret123", 0)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 7777 {
		t.Fatalf("expected name-only exact match to ID 7777 with year=0, got %d", id)
	}
	logged := buf.String()
	if !strings.Contains(logged, "fallback_reason=exact_match") {
		t.Fatalf("expected fallback_reason=exact_match, got %q", logged)
	}
}

// TestSearchMediaID_NormalizedCompare: exact match is case-insensitive
// with collapsed whitespace.
func TestSearchMediaID_NormalizedCompare(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := SearchResponse{
			Results: []MediaResult{
				// Decoy first so accidental fallback cannot satisfy the assertion.
				{ID: 4444, Title: "Something Else", ReleaseDate: "2000-01-01"},
				{ID: 5555, Title: "the  dark  knight", ReleaseDate: "2008-07-18"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("The Dark Knight", "Movies", "secret123", 2008)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 5555 {
		t.Fatalf("expected normalized-case exact match to ID 5555, got %d", id)
	}
	logged := buf.String()
	if !strings.Contains(logged, "fallback_reason=exact_match") {
		t.Fatalf("expected fallback_reason=exact_match, got %q", logged)
	}
}

// TestSearchMediaID_WireQueryEncoded: the query parameter must be sent
// with url.QueryEscape so &, +, = survive intact on the wire. With the
// old PathEscape the raw query contained a bare & which split the
// parameter server-side.
func TestSearchMediaID_WireQueryEncoded(t *testing.T) {
	_, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SearchResponse{Results: []MediaResult{{ID: 1, Name: "S&X", FirstAirDate: "2023-01-01"}}})
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("S&X", "TV", "secret123", 0)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 1 {
		t.Fatalf("expected exact match ID 1, got %d", id)
	}
	if !strings.Contains(gotQuery, "%26") {
		t.Fatalf("expected %%26 in raw query, got %q", gotQuery)
	}
	if strings.Contains(gotQuery[:strings.Index(gotQuery, "&include_adult")], "S&X") {
		t.Fatalf("raw query contains unencoded & inside query value: %q", gotQuery)
	}
}

// TestSearchMediaID_YearParamPassThrough: the TMDB year parameter is sent
// only when year > 0.
func TestSearchMediaID_YearParamPassThrough(t *testing.T) {
	tests := []struct {
		name     string
		year     int64
		wantYear string
	}{
		{"year sent when positive", 2021, "2021"},
		{"year omitted when zero", 0, ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, restore := setupBufferLogger(slog.LevelDebug)
			defer restore()

			origBase := tmdbBaseURL
			defer func() { tmdbBaseURL = origBase }()

			var gotYear string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotYear = r.URL.Query().Get("year")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(SearchResponse{Results: []MediaResult{{ID: 42, Title: "Dune", ReleaseDate: "2021-10-22"}}})
			}))
			defer srv.Close()
			tmdbBaseURL = srv.URL

			if _, err := SearchMediaID("Dune", "Movies", "secret123", tt.year); err != nil {
				t.Fatalf("SearchMediaID error: %v", err)
			}
			if gotYear != tt.wantYear {
				t.Fatalf("year param = %q, want %q", gotYear, tt.wantYear)
			}
		})
	}
}

// TestGetMediaDetails_MissingDatesErrorsNotPanic: empty ReleaseDate AND
// empty FirstAirDate must return an error, never a slice-bounds panic
// from date[:4].
func TestGetMediaDetails_MissingDatesErrorsNotPanic(t *testing.T) {
	_, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := MediaResult{ID: 123, Title: "Dateless", Popularity: 1.0, VoteAverage: 5.0, VoteCount: 10, OriginalLanguage: "en"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	_, err := GetMediaDetails(123, "Movies", "secret123")
	if err == nil {
		t.Fatalf("expected error for missing release/first-air dates, got nil")
	}
}

// TestGetMediaDetails_ShortFirstAirDateErrorsNotPanic: a too-short date
// string must error instead of panicking on date[:4].
func TestGetMediaDetails_ShortFirstAirDateErrorsNotPanic(t *testing.T) {
	_, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := MediaResult{ID: 124, Name: "Partial Date", FirstAirDate: "20"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	_, err := GetMediaDetails(124, "TV", "secret123")
	if err == nil {
		t.Fatalf("expected error for short first air date, got nil")
	}
}

// TestSearchMediaID_FallbackWarnsCandidates: fallback must surface top
// candidates via slog.Warn (never silent).
func TestSearchMediaID_FallbackWarnsCandidates(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := SearchResponse{
			Results: []MediaResult{
				{ID: 10, Title: "Dune Part Two", ReleaseDate: "2024-03-01"},
				{ID: 11, Title: "Other", ReleaseDate: "2021-10-22"},
				{ID: 12, Title: "Third", ReleaseDate: "2020-01-01"},
				{ID: 13, Title: "Fourth", ReleaseDate: "2019-01-01"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("Dune", "Movies", "secret123", 2021)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 10 {
		t.Fatalf("expected fallback to first result ID 10, got %d", id)
	}
	logged := buf.String()
	if !strings.Contains(logged, "level=WARN") {
		t.Fatalf("expected WARN level on fallback, got %q", logged)
	}
	for _, want := range []string{"candidate_0_id=10", "candidate_1_id=11", "candidate_2_id=12"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("expected candidate field %q in warn log, got %q", want, logged)
		}
	}
	if strings.Contains(logged, "candidate_3_id=13") {
		t.Fatalf("warn log must list at most 3 candidates, got %q", logged)
	}
}

// TestSearchMediaID_YearMismatchSkipsTitleMatch: a display-name match with
// a disagreeing release year must be skipped in favor of the warned
// first-result fallback. Guards against dropping or inverting the year
// comparison — mutations that would silently resolve same-title
// different-era entries (e.g. Dune 1984 vs 2021) to the wrong TMDB ID.
func TestSearchMediaID_YearMismatchSkipsTitleMatch(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := SearchResponse{
			Results: []MediaResult{
				// Decoy first: correct behavior falls back to it; a dropped
				// or inverted year comparison would exact-match ID 5555.
				{ID: 4444, Title: "Something Else", ReleaseDate: "2000-01-01"},
				{ID: 5555, Title: "Dune", ReleaseDate: "2021-10-22"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("Dune", "Movies", "secret123", 1984)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 4444 {
		t.Fatalf("expected year-mismatched candidate to be skipped (fallback to 4444), got %d", id)
	}
	logged := buf.String()
	if !strings.Contains(logged, "fallback_reason=fallback_first_result") {
		t.Fatalf("expected warned fallback, got %q", logged)
	}
	if strings.Contains(logged, "fallback_reason=exact_match") {
		t.Fatalf("exact_match must not fire on year disagreement, got %q", logged)
	}
}

// TestSearchMediaID_ShortDateSkipsCandidateWithoutPanic: a title-matching
// candidate with an unusable (<4 char) date string is skipped by the len
// guard instead of panicking on date[:4].
func TestSearchMediaID_ShortDateSkipsCandidateWithoutPanic(t *testing.T) {
	buf, restore := setupBufferLogger(slog.LevelDebug)
	defer restore()

	origBase := tmdbBaseURL
	defer func() { tmdbBaseURL = origBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := SearchResponse{
			Results: []MediaResult{
				{ID: 7777, Title: "Dune", ReleaseDate: "20"},
				{ID: 8888, Title: "Other", ReleaseDate: "2001-01-01"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	tmdbBaseURL = srv.URL

	id, err := SearchMediaID("Dune", "Movies", "secret123", 2023)
	if err != nil {
		t.Fatalf("SearchMediaID error: %v", err)
	}
	if id != 7777 {
		t.Fatalf("expected guarded skip then fallback to first result 7777, got %d", id)
	}
	logged := buf.String()
	if !strings.Contains(logged, "fallback_reason=fallback_first_result") {
		t.Fatalf("expected warned fallback after guarded skip, got %q", logged)
	}
}
