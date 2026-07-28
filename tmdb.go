package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

const tmdbBaseURL = "https://api.themoviedb.org/3"

func SearchMediaID(title, mediaType, tmdbAPIKey string, year int64) (int64, error) {
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
		return 0, fmt.Errorf("no media found for query: %s", title)
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

func GetMediaDetails(id int64, mediaType, tmdbAPIKey string) (MediaResult, error) {
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

	if mediaDetails.ReleaseDate == "" {
		mediaDetails.ReleaseYear, err = strconv.ParseInt(mediaDetails.FirstAirDate[:4], 10, 64)
	} else {
		mediaDetails.ReleaseYear, err = strconv.ParseInt(mediaDetails.ReleaseDate[:4], 10, 64)
	}

	if err != nil {
		return MediaResult{}, fmt.Errorf("failed to parse release year: %w", err)
	}

	return mediaDetails, nil
}

func GetMediaDetailsWorkflow(title, mediaType, tmdbAPIKey string, year int64) (MediaResult, error) {
	id, err := SearchMediaID(title, mediaType, tmdbAPIKey, year)
	if err != nil {
		return MediaResult{}, fmt.Errorf("failed to search media ID: %w", err)
	}

	mediaDetails, err := GetMediaDetails(id, mediaType, tmdbAPIKey)
	if err != nil {
		return MediaResult{}, fmt.Errorf("failed to get media details: %w", err)
	}

	return mediaDetails, nil
}

func doTMDBGet(url, tmdbAPIKey string, target any) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tmdbAPIKey))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
