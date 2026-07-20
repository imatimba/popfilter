package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	os.Args = preprocessArgs(os.Args)

	titlePtr := flag.String("title", "", "Required. Title of the movie to search for.")
	yearPtr := flag.Int64("year", 0, "Required. Year of release.")
	releaseGroupPtr := flag.String("release-group", "", "Required. Release group of the movie to search for.")
	videoResolutionPtr := flag.String("video-resolution", "", "Required. Video resolution of the movie to search for.")

	flag.Parse()

	tmdbAPIKey, err := getTMDBAPIKey()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if *titlePtr == "" || *releaseGroupPtr == "" || *yearPtr == 0 || *videoResolutionPtr == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	cfg, err := getConfig(fmt.Sprintf("%s-%s", *releaseGroupPtr, *videoResolutionPtr))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	movieDetails, err := GetMovieDetailsWorkflow(*titlePtr, tmdbAPIKey, *yearPtr)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	genres := make([]string, len(movieDetails.Genres))
	for i, genre := range movieDetails.Genres {
		genres[i] = genre.Name
	}

	score := cfg.ComputeScore(movieDetails.Popularity,
		movieDetails.VoteAverage,
		movieDetails.VoteCount,
		*yearPtr,
		movieDetails.OriginalLanguage,
		*releaseGroupPtr,
		genres)

	fmt.Printf("Score for %s [%s] %s: %f\n", movieDetails.Title, *releaseGroupPtr, *videoResolutionPtr, score)

	if score >= cfg.scoreThreshold {
		fmt.Println("Exiting with status code 0")
		os.Exit(0)
	}

	fmt.Println("Exiting with status code 1")
	os.Exit(1)
}
