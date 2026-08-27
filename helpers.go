package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func getTMDBAPIKey(argAPIKey *string) (string, string, error) {
	// 1. Explicit env var (most portable)
	if key := os.Getenv("TMDB_API_KEY"); key != "" {
		slog.Info("tmdb key resolved", "key_source", "env", "key_present", true)
		return key, "env", nil
	}
	// 2. Local .env for dev convenience
	_ = godotenv.Load()
	if key := os.Getenv("TMDB_API_KEY"); key != "" {
		slog.Info("tmdb key resolved", "key_source", "dotenv", "key_present", true)
		return key, "dotenv", nil
	}
	// 3. Optional arg
	if argAPIKey != nil && *argAPIKey != "" {
		slog.Info("tmdb key resolved", "key_source", "flag", "key_present", true)
		return *argAPIKey, "flag", nil
	}

	slog.Error("tmdb key missing", "key_source", "missing", "key_present", false)
	return "", "missing", fmt.Errorf("TMDB_API_KEY not set (env var, .env or --tmdb-api-key)")
}

func getMediaType(mediaType string) (string, error) {
	switch mediaType {
	case "Movies":
		mediaType = "movie"
	case "TV":
		mediaType = "tv"
	default:
		return "", fmt.Errorf("invalid media type: %s", mediaType)
	}
	return mediaType, nil
}
