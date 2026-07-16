package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func main() {
	titlePtr := flag.String("title", "", "Required. Title of the movie to search for.")
	yearPtr := flag.Int64("year", 0, "Required. Year of release.")
	releaseGroupPtr := flag.String("release-group", "", "Required. Release group of the movie to search for.")
	flag.Parse()

	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		os.Exit(1)
	}

	tmdbAPIKey := os.Getenv("TMDB_API_KEY")
	if tmdbAPIKey == "" {
		fmt.Println("Error: TMDB_API_KEY not set in .env file")
		os.Exit(1)
	}

	if *titlePtr == "" || *releaseGroupPtr == "" || *yearPtr == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	var scoreThreshold float64

	switch *releaseGroupPtr {
	case "BHDStudio":
		scoreThreshold = 2.520739041141921
	case "FraMeSToR":
		scoreThreshold = 2.697175367320239
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

	score := ComputeScore(movieDetails.Popularity,
		movieDetails.VoteAverage,
		movieDetails.VoteCount,
		year,
		movieDetails.OriginalLanguage,
		*releaseGroupPtr,
		genres)

	fmt.Printf("Score: %f\n", score)

	if score >= scoreThreshold {
		os.Exit(0)
	}

	os.Exit(1)
}
