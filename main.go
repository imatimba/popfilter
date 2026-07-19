package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
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

	year, err := strconv.ParseInt(movieDetails.ReleaseDate[:4], 10, 64)
	if err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}

	score := cfg.ComputeScore(movieDetails.Popularity,
		movieDetails.VoteAverage,
		movieDetails.VoteCount,
		year,
		movieDetails.OriginalLanguage,
		*releaseGroupPtr,
		genres)

	fmt.Printf("Score: %f\n", score)

	if score >= cfg.scoreThreshold {
		os.Exit(0)
	}

	os.Exit(1)
}
