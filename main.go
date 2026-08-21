package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
)

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "popfilter - evaluate torrent release desirability using TMDB features")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage: popfilter [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	prev := flag.CommandLine.Output()
	flag.CommandLine.SetOutput(w)
	flag.PrintDefaults()
	flag.CommandLine.SetOutput(prev)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Environment:")
	fmt.Fprintln(w, "  TMDB_API_KEY   API key; resolution order is env var > .env file > --tmdb-api-key flag")
	fmt.Fprintln(w, "  TMDB_BASE_URL  Overrides the TMDB API base URL (default https://api.themoviedb.org/3); mainly for testing/proxies")
}

func main() {
	os.Args = preprocessArgs(os.Args)

	flag.Usage = func() { printUsage(os.Stderr) }

	titlePtr := flag.String("title", "", "Required. Title of the movie to search for.")
	yearPtr := flag.Int64("year", 0, "Optional. Year of release.")
	mediaTypePtr := flag.String("media-type", "", "Required. Media type of the movie to search for.")
	releaseGroupPtr := flag.String("release-group", "", "Required. Release group of the movie to search for.")
	videoResolutionPtr := flag.String("video-resolution", "", "Required. Video resolution of the movie to search for.")
	tmdbAPIKeyPtr := flag.String("tmdb-api-key", "", "Optional. TMDB API key.")
	logLevelPtr := flag.String("log-level", "info", "Log level: debug|info|warn|error (default info, case-insensitive).")
	logFilePtr := flag.String("log-file", "", "Optional. Append structured logs to file (O_APPEND 0644) in addition to stderr. Parent dir must exist.")

	flag.Parse()

	logger, logFile, err := NewLogger(*logLevelPtr, *logFilePtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		flag.Usage()
		os.Exit(1)
	}
	if logFile != nil {
		defer logFile.Close()
	}
	slog.SetDefault(logger)

	tmdbAPIKey, keySource, err := getTMDBAPIKey(tmdbAPIKeyPtr)
	if err != nil {
		slog.Error("failed to resolve tmdb key", "error", err, "title", *titlePtr, "media_type", *mediaTypePtr, "key_source", keySource)
		os.Exit(1)
	}

	if *titlePtr == "" || *releaseGroupPtr == "" || *mediaTypePtr == "" || *videoResolutionPtr == "" {
		fmt.Fprintln(os.Stderr, "Error: missing required flags: -title, -media-type, -release-group, -video-resolution")
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := getConfig(fmt.Sprintf("%s-%s-%s", *releaseGroupPtr, *mediaTypePtr, *videoResolutionPtr))
	if err != nil {
		slog.Error("unsupported release group", "error", err, "title", *titlePtr, "media_type", *mediaTypePtr, "release_group", *releaseGroupPtr)
		os.Exit(1)
	}

	if base := os.Getenv("TMDB_BASE_URL"); base != "" {
		tmdbBaseURL = base
	}

	mediaDetails, err := GetMediaDetailsWorkflow(*titlePtr, *mediaTypePtr, tmdbAPIKey, *yearPtr)
	if err != nil {
		slog.Error("tmdb workflow failed", "error", err, "title", *titlePtr, "media_type", *mediaTypePtr, "year", *yearPtr)
		os.Exit(1)
	}

	genres := make([]string, len(mediaDetails.Genres))
	for i, genre := range mediaDetails.Genres {
		genres[i] = genre.Name
	}

	score := cfg.ComputeScore(mediaDetails.Popularity,
		mediaDetails.VoteAverage,
		mediaDetails.VoteCount,
		mediaDetails.ReleaseYear,
		mediaDetails.OriginalLanguage,
		*releaseGroupPtr,
		genres)

	decision := "reject"
	if score >= cfg.scoreThreshold {
		decision = "keep"
	}
	logScoringDecision(cfg, mediaDetails, *titlePtr, *mediaTypePtr, *yearPtr, *releaseGroupPtr, *videoResolutionPtr, keySource, decision, score, genres)

	fmt.Printf("Score for %s [%s] %s: %f\n", mediaDetails.Title, *releaseGroupPtr, *videoResolutionPtr, score)

	attrs := []any{
		"decision", decision,
		"score", score,
		"threshold", cfg.scoreThreshold,
		"title", *titlePtr,
		"media_type", *mediaTypePtr,
		"release_group", *releaseGroupPtr,
		"video_resolution", *videoResolutionPtr,
		"tmdb_id", mediaDetails.ID,
		"resolved_title", mediaDetails.Title,
	}
	slog.Info("exiting", attrs...)
	if decision == "keep" {
		os.Exit(0)
	}
	os.Exit(1)
}

func logScoringDecision(cfg scoringConfig, mediaDetails MediaResult, title, mediaType string, year int64, releaseGroup, videoResolution, keySource string, decision string, score float64, genres []string) {
	slog.Info("score decision",
		"title", title,
		"media_type", mediaType,
		"year", year,
		"release_group", releaseGroup,
		"video_resolution", videoResolution,
		"tmdb_id", mediaDetails.ID,
		"resolved_title", mediaDetails.Title,
		"score", score,
		"threshold", cfg.scoreThreshold,
		"decision", decision,
		"key_source", keySource,
	)
	slog.Debug("score inputs",
		"title", title,
		"media_type", mediaType,
		"popularity", mediaDetails.Popularity,
		"vote_average", mediaDetails.VoteAverage,
		"vote_count", mediaDetails.VoteCount,
		"release_year", mediaDetails.ReleaseYear,
		"original_language", mediaDetails.OriginalLanguage,
		"genres", genres,
		"weights", fmt.Sprintf("wVc=%.4f wPop=%.4f wVa=%.4f wEn=%.4f wYear=%.4f", cfg.wVc, cfg.wPop, cfg.wVa, cfg.wEn, cfg.wYear),
		"score", score,
		"threshold", cfg.scoreThreshold,
		"decision", decision,
		"tmdb_id", mediaDetails.ID,
		"resolved_title", mediaDetails.Title,
	)
}
