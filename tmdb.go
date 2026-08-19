package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type MediaResult struct {
	Title            string  `json:"title,omitempty"`
	Name             string  `json:"name,omitempty"`
	OriginalLanguage string  `json:"original_language"`
	ID               int64   `json:"id"`
	Popularity       float64 `json:"popularity"`
	VoteCount        int64   `json:"vote_count"`
	VoteAverage      float64 `json:"vote_average"`
	ReleaseDate      string  `json:"release_date,omitempty"`
	FirstAirDate     string  `json:"first_air_date,omitempty"`
	ReleaseYear      int64
	Genres           []Genre `json:"genres,omitempty"`
}

type SearchResponse struct {
	Results []MediaResult `json:"results"`
}

var tmdbBaseURL = "https://api.themoviedb.org/3"

func SearchMediaID(title, mediaType, tmdbAPIKey string, year int64) (int64, error) {
	authPresent := tmdbAPIKey != ""
	slog.Debug("tmdb search", "title", title, "media_type", mediaType, "year", year, "tmdb_search_query", title, "attempt", 1, "auth_present", authPresent)

	mediaType, err := getMediaType(mediaType)
	if err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/search/%s?query=%s&include_adult=true", tmdbBaseURL, mediaType, url.PathEscape(title))

	var searchResp SearchResponse

	err = doTMDBGet(url, tmdbAPIKey, &searchResp)
	if err != nil {
		return 0, err
	}

	if len(searchResp.Results) == 0 {
		slog.Error("tmdb search no results", "title", title, "media_type", mediaType, "year", year, "tmdb_search_query", title, "attempt", 1, "fallback_reason", "no_result_error")
		return 0, fmt.Errorf("no media found for query: %s", title)
	}

	for _, result := range searchResp.Results {
		if result.ReleaseDate == "" {
			continue
		}

		releaseYear := result.ReleaseDate[:4]
		if result.Title == title && releaseYear == fmt.Sprintf("%d", year) {
			slog.Info("tmdb search result", "title", title, "media_type", mediaType, "year", year, "tmdb_search_query", title, "tmdb_id", result.ID, "fallback_reason", "exact_match", "attempt", 1)
			return result.ID, nil
		}
	}

	slog.Info("tmdb search result", "title", title, "media_type", mediaType, "year", year, "tmdb_search_query", title, "tmdb_id", searchResp.Results[0].ID, "fallback_reason", "fallback_first_result", "attempt", 1)
	return searchResp.Results[0].ID, nil
}

func GetMediaDetails(id int64, mediaType, tmdbAPIKey string) (MediaResult, error) {
	authPresent := tmdbAPIKey != ""
	slog.Debug("tmdb details request", "tmdb_id", id, "media_type", mediaType, "attempt", 1, "auth_present", authPresent)

	mediaType, err := getMediaType(mediaType)
	if err != nil {
		return MediaResult{}, err
	}

	url := fmt.Sprintf("%s/%s/%d", tmdbBaseURL, mediaType, id)

	var mediaDetails MediaResult

	err = doTMDBGet(url, tmdbAPIKey, &mediaDetails)
	if err != nil {
		return MediaResult{}, err
	}

	if mediaDetails.Title == "" {
		mediaDetails.Title = mediaDetails.Name
	}

	// Resolve title for logging before parsing year
	resolvedTitle := mediaDetails.Title

	if mediaDetails.ReleaseDate == "" {
		mediaDetails.ReleaseYear, err = strconv.ParseInt(mediaDetails.FirstAirDate[:4], 10, 64)
	} else {
		mediaDetails.ReleaseYear, err = strconv.ParseInt(mediaDetails.ReleaseDate[:4], 10, 64)
	}

	if err != nil {
		slog.Error("tmdb details parse failed", "tmdb_id", id, "media_type", mediaType, "error", err, "attempt", 1)
		return MediaResult{}, fmt.Errorf("failed to parse release year: %w", err)
	}

	slog.Info("tmdb details", "tmdb_id", id, "media_type", mediaType, "resolved_title", resolvedTitle, "release_year", mediaDetails.ReleaseYear, "attempt", 1)
	return mediaDetails, nil
}

func GetMediaDetailsWorkflow(title, mediaType, tmdbAPIKey string, year int64) (MediaResult, error) {
	slog.Debug("tmdb workflow start", "title", title, "media_type", mediaType, "year", year, "attempt", 1)
	id, err := SearchMediaID(title, mediaType, tmdbAPIKey, year)
	if err != nil {
		slog.Error("tmdb workflow search failed", "title", title, "media_type", mediaType, "year", year, "error", err, "attempt", 1)
		return MediaResult{}, fmt.Errorf("failed to search media ID: %w", err)
	}

	mediaDetails, err := GetMediaDetails(id, mediaType, tmdbAPIKey)
	if err != nil {
		slog.Error("tmdb workflow details failed", "title", title, "media_type", mediaType, "tmdb_id", id, "error", err, "attempt", 1)
		return MediaResult{}, fmt.Errorf("failed to get media details: %w", err)
	}

	return mediaDetails, nil
}

func doTMDBGet(url, tmdbAPIKey string, target any) error {
	start := time.Now()
	authPresent := tmdbAPIKey != ""
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("tmdb request create failed", "url_path", redactURLPath(url), "error", err, "attempt", 1, "auth_present", authPresent)
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tmdbAPIKey))
	req.Header.Set("Accept", "application/json")

	slog.Debug("tmdb request", "url_path", redactURLPath(url), "auth_present", authPresent, "attempt", 1)

	resp, err := client.Do(req)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		slog.Error("tmdb request failed", "url_path", redactURLPath(url), "duration_ms", durationMs, "error", err, "attempt", 1, "auth_present", authPresent)
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	if statusCode != http.StatusOK {
		slog.Error("tmdb api error", "url_path", redactURLPath(url), "status_code", statusCode, "duration_ms", durationMs, "attempt", 1, "auth_present", authPresent)
		return fmt.Errorf("API error: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		slog.Error("tmdb decode failed", "url_path", redactURLPath(url), "status_code", statusCode, "duration_ms", durationMs, "error", err, "attempt", 1)
		return fmt.Errorf("failed to decode response: %w", err)
	}

	slog.Debug("tmdb response", "url_path", redactURLPath(url), "status_code", statusCode, "duration_ms", durationMs, "attempt", 1, "auth_present", authPresent)
	return nil
}

func redactURLPath(raw string) string {
	if idx := strings.Index(raw, "?"); idx != -1 {
		return raw[:idx]
	}
	return raw
}
