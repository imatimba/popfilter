package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type MovieResult struct {
	Title            string  `json:"title"`
	OriginalLanguage string  `json:"original_language"`
	ID               int64   `json:"id"`
	Popularity       float64 `json:"popularity"`
	VoteCount        int64   `json:"vote_count"`
	VoteAverage      float64 `json:"vote_average"`
	ReleaseDate      string  `json:"release_date,omitempty"`
	Genres           []Genre `json:"genres,omitempty"`
}

type SearchResponse struct {
	Results []MovieResult `json:"results"`
}

const tmdbBaseURL = "https://api.themoviedb.org/3"

func SearchMovieID(title, tmdbAPIKey string, year int64) (int64, error) {
	url := fmt.Sprintf("%s/search/movie?query=%s&include_adult=true", tmdbBaseURL, url.PathEscape(title))

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tmdbAPIKey))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API error: %s", resp.Status)
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.Results) == 0 {
		return 0, fmt.Errorf("no movie found for query: %s", title)
	}

	for _, result := range searchResp.Results {
		if result.ReleaseDate == "" {
			continue
		}

		releaseYear := result.ReleaseDate[:4]
		if result.Title == title && releaseYear == fmt.Sprintf("%d", year) {
			return result.ID, nil
		}
	}

	return searchResp.Results[0].ID, nil
}

func GetMovieDetails(id int64, tmdbAPIKey string) (MovieResult, error) {
	url := fmt.Sprintf("%s/movie/%d", tmdbBaseURL, id)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return MovieResult{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tmdbAPIKey))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return MovieResult{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return MovieResult{}, fmt.Errorf("API error: %s", resp.Status)
	}

	var movieDetails MovieResult
	if err := json.NewDecoder(resp.Body).Decode(&movieDetails); err != nil {
		return MovieResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return movieDetails, nil
}

func GetMovieDetailsWorkflow(title, tmdbAPIKey string, year int64) (MovieResult, error) {
	id, err := SearchMovieID(title, tmdbAPIKey, year)
	if err != nil {
		return MovieResult{}, fmt.Errorf("failed to search movie ID: %w", err)
	}

	movieDetails, err := GetMovieDetails(id, tmdbAPIKey)
	if err != nil {
		return MovieResult{}, fmt.Errorf("failed to get movie details: %w", err)
	}

	return movieDetails, nil
}
