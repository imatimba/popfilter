package main

import (
	"math"
)

// scoringConfig holds per-dataset weights for the additive scoring formula.
type scoringConfig struct {
	wVc   float64
	wPop  float64
	wVa   float64
	wEn   float64
	wYear float64
}

var (
	bhdstudioConfig = scoringConfig{
		wVc:   2.2900,
		wPop:  1.3500,
		wVa:   0.5300,
		wEn:   0.4000,
		wYear: 0.0600,
	}
	framestorConfig = scoringConfig{
		wVc:   2.4900,
		wPop:  -0.5400,
		wVa:   1.5200,
		wEn:   0.2000,
		wYear: 0.0900,
	}
)

const (
	maxPop             float64 = 3000.0
	maxVa              float64 = 10.0
	maxVc              float64 = 150000.0
	newReleaseBaseYear int64   = 2020

	// Genre modifiers (additive)
	wHorror  float64 = -0.10
	wHistory float64 = 0.05
	wFamily  float64 = 0.05
)

func ComputeScore(popularity, voteAverage float64,
	voteCount, year int64,
	language, releaseGroup string,
	genres []string) float64 {

	// Select config based on release group
	var cfg scoringConfig
	switch releaseGroup {
	case "BHDStudio":
		cfg = bhdstudioConfig
	case "FraMeSToR":
		cfg = framestorConfig
	}

	logMaxPop := math.Log1p(maxPop)
	logMaxVc := math.Log1p(maxVc)

	popNorm := math.Max(math.Log1p(popularity)/logMaxPop, 1e-9)
	vaNorm := math.Max(voteAverage/maxVa, 1e-9)
	vcNorm := math.Max(math.Log1p(float64(voteCount))/logMaxVc, 1e-9)

	score := (cfg.wVc * vcNorm) + (cfg.wPop * popNorm) + (cfg.wVa * vaNorm)

	// Recency boost
	if year > newReleaseBaseYear {
		score += cfg.wYear * float64(year-newReleaseBaseYear)
	}

	// Language modifier
	if language == "en" {
		score += cfg.wEn
	}

	// Genre modifiers
	for _, genre := range genres {
		switch genre {
		case "Horror":
			score += wHorror
		case "History":
			score += wHistory
		case "Family":
			score += wFamily
		}
	}

	return score
}
