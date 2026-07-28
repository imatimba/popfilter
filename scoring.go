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
	"BHDStudio-Movies-2160p": {
		scoreThreshold: 2.520739041141921,
		wVc:            2.2900,
		wPop:           1.3500,
		wVa:            0.5300,
		wEn:            0.4000,
		wYear:          0.0600,
	},
	"BHDStudio-Movies-1080p": {
		scoreThreshold: 3.1163,
		wVc:            3.3650,
		wPop:           1.8550,
		wVa:            0.2000,
		wEn:            0.6000,
		wYear:          0.1500,
	},
	"FraMeSToR-Movies-2160p": {
		scoreThreshold: 2.697175367320239,
		wVc:            2.4900,
		wPop:           -0.5400,
		wVa:            1.5200,
		wEn:            0.2000,
		wYear:          0.0900,
	},
	"FraMeSToR-Movies-1080p": {
		scoreThreshold: 3.2661,
		wVc:            3.4200,
		wPop:           0.1000,
		wVa:            1.6500,
		wEn:            0.2000,
		wYear:          0.0900,
	},
	"BYNDR-Movies-2160p": {
		scoreThreshold: 1.7245,
		wVc:            2.0000,
		wPop:           -0.1000,
		wVa:            0.0000,
		wEn:            0.0000,
		wYear:          0.1500,
	},
	"BYNDR-Movies-1080p": {
		scoreThreshold: 1.8645,
		wVc:            3.3500,
		wPop:           1.0600,
		wVa:            0.4000,
		wEn:            0.2000,
		wYear:          0.0000,
	},
	"FLUX-Movies-2160p": {
		scoreThreshold: 2.3110,
		wVc:            2.2200,
		wPop:           0.4400,
		wVa:            0.7800,
		wEn:            0.2000,
		wYear:          0.1200,
	},
	"FLUX-Movies-1080p": {
		scoreThreshold: 2.5128,
		wVc:            3.0700,
		wPop:           -0.6000,
		wVa:            0.7800,
		wEn:            0.6000,
		wYear:          0.1500,
	},
	"FLUX-TV-2160p": {
		scoreThreshold: 0.9165,
		wVc:            1.2500,
		wPop:           0.2100,
		wVa:            0.0000,
		wEn:            0.2000,
		wYear:          0.0300,
	},
	"FLUX-TV-1080p": {
		scoreThreshold: 2.5803,
		wVc:            3.2000,
		wPop:           0.2000,
		wVa:            0.4900,
		wEn:            0.4000,
		wYear:          0.1500,
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
func (cfg *scoringConfig) ComputeScore(popularity, voteAverage float64,
	voteCount, year int64,
	language, releaseGroup string,
	genres []string) float64 {
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
