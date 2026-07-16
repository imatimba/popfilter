package main

import (
	"fmt"
	"math"
)

// scoringConfig holds per-dataset weights and decision threshold for the additive scoring formula.
type scoringConfig struct {
	scoreThreshold float64
	wVc            float64
	wPop           float64
	wVa            float64
	wEn            float64
	wYear          float64
}

var configs = map[string]scoringConfig{
	"BHDStudio": {
		scoreThreshold: 2.520739041141921,
		wVc:            2.2900,
		wPop:           1.3500,
		wVa:            0.5300,
		wEn:            0.4000,
		wYear:          0.0600,
	},
	"FraMeSToR": {
		scoreThreshold: 2.697175367320239,
		wVc:            2.4900,
		wPop:           -0.5400,
		wVa:            1.5200,
		wEn:            0.2000,
		wYear:          0.0900,
	},
}

const (
	// Normalization constants
	maxPop             float64 = 3000.0
	maxVa              float64 = 10.0
	maxVc              float64 = 150000.0
	newReleaseBaseYear int64   = 2020

	// Genre modifiers (additive)
	wHorror  float64 = -0.10
	wHistory float64 = 0.05
	wFamily  float64 = 0.05
)

// getConfig returns the scoring configuration for a supported release group.
func getConfig(releaseGroup string) (scoringConfig, error) {
	cfg, ok := configs[releaseGroup]
	if !ok {
		return scoringConfig{}, fmt.Errorf("unsupported release group: %s", releaseGroup)
	}
	return cfg, nil
}

// ComputeScore returns a desirability score from TMDB features, release year,
// language, and genres using a weighted additive model.
func ComputeScore(popularity, voteAverage float64,
	voteCount, year int64,
	language, releaseGroup string,
	genres []string) float64 {

	cfg, err := getConfig(releaseGroup)
	if err != nil {
		return 0.0
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

	// Genre modifiers (membership-based, each applied at most once)
	genreSet := make(map[string]struct{}, len(genres))
	for _, g := range genres {
		genreSet[g] = struct{}{}
	}
	if _, ok := genreSet["Horror"]; ok {
		score += wHorror
	}
	if _, ok := genreSet["History"]; ok {
		score += wHistory
	}
	if _, ok := genreSet["Family"]; ok {
		score += wFamily
	}

	return score
}
