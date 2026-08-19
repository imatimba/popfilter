package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// preprocessArgs merges a bare (unquoted) -title value spread across
// multiple argv tokens into a single token so flag.Parse() handles it.
// e.g. ["-title", "The", "Dark", "Knight", "-year", "2008"]
//
//	-> ["-title", "The Dark Knight", "-year", "2008"]
func preprocessArgs(args []string) []string {
	// Flags that consume a value. We stop collecting title words when we
	// hit one of these.
	valueFlags := map[string]bool{
		"-title":             true,
		"--title":            true,
		"-year":              true,
		"--year":             true,
		"-media-type":        true,
		"--media-type":       true,
		"-release-group":     true,
		"--release-group":    true,
		"-video-resolution":  true,
		"--video-resolution": true,
		"-tmdb-api-key":      true,
		"--tmdb-api-key":     true,
		"-log-level":         true,
		"--log-level":        true,
		"-log-file":          true,
		"--log-file":         true,
	}

	var result []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "-title" || arg == "--title" {
			result = append(result, arg)
			i++
			var titleParts []string
			for i < len(args) && !valueFlags[args[i]] {
				titleParts = append(titleParts, args[i])
				i++
			}
			if len(titleParts) > 0 {
				result = append(result, strings.Join(titleParts, " "))
			}
			continue
		}
		result = append(result, arg)
		i++
	}
	return result
}

func getTMDBAPIKey(argAPIKey *string) (string, string, bool, error) {
	// 1. Explicit env var (most portable)
	if key := os.Getenv("TMDB_API_KEY"); key != "" {
		slog.Info("tmdb key resolved", "key_source", "env", "key_present", true)
		return key, "env", true, nil
	}
	// 2. Local .env for dev convenience
	_ = godotenv.Load()
	if key := os.Getenv("TMDB_API_KEY"); key != "" {
		slog.Info("tmdb key resolved", "key_source", "dotenv", "key_present", true)
		return key, "dotenv", true, nil
	}
	// 3. Optional arg
	if argAPIKey != nil && *argAPIKey != "" {
		slog.Info("tmdb key resolved", "key_source", "flag", "key_present", true)
		return *argAPIKey, "flag", true, nil
	}

	slog.Error("tmdb key missing", "key_source", "missing", "key_present", false)
	return "", "missing", false, fmt.Errorf("TMDB_API_KEY not set (env var, .env or --tmdb-api-key)")
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
