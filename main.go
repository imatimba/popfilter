package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	os.Args = preprocessArgs(os.Args)

	titlePtr := flag.String("title", "", "Required. Title of the movie to search for.")
	yearPtr := flag.Int64("year", 0, "Optional. Year of release.")
	mediaTypePtr := flag.String("media-type", "", "Required. Media type of the movie to search for.")
	releaseGroupPtr := flag.String("release-group", "", "Required. Release group of the movie to search for.")
	videoResolutionPtr := flag.String("video-resolution", "", "Required. Video resolution of the movie to search for.")
	tmdbAPIKeyPtr := flag.String("tmdb-api-key", "", "Optional. TMDB API key.")

	flag.Parse()

	tmdbAPIKey, err := getTMDBAPIKey(tmdbAPIKeyPtr)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if *titlePtr == "" || *releaseGroupPtr == "" || *mediaTypePtr == "" || *videoResolutionPtr == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	cfg, err := getConfig(fmt.Sprintf("%s-%s-%s", *releaseGroupPtr, *mediaTypePtr, *videoResolutionPtr))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	mediaDetails, err := GetMediaDetailsWorkflow(*titlePtr, *mediaTypePtr, tmdbAPIKey, *yearPtr)
	if err != nil {
		fmt.Println("Error:", err)
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

	fmt.Printf("Score for %s [%s] %s: %f\n", mediaDetails.Title, *releaseGroupPtr, *videoResolutionPtr, score)

	if score >= cfg.scoreThreshold {
		fmt.Println("Exiting with status code 0")
		os.Exit(0)
	}

	fmt.Println("Exiting with status code 1")
	os.Exit(1)
}
