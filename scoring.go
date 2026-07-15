package main

import (
	"math"
)

func ComputeScore(popularity, voteAverage float64,
	voteCount, year int64,
	language, releaseGroup string,
	genres []string) float64 {

	const (
		// Normalization constants
		maxPop float64 = 3000.0
		maxVa  float64 = 10.0
		maxVc  float64 = 150000.0

		// Recency Boost (adds +X% to score per year after the base year)
		newReleaseBoost    float64 = 0.02
		newReleaseBaseYear int64   = 2020

		// Modifiers that had the most impact on the score (tuned via grid search, 2026-07-15)
		enBoost       float64 = 1.08
		historyBoost  float64 = 1.07
		horrorPenalty float64 = 0.92
	)

	// Exponents for the weighted geometric mean (tuned via grid search, 2026-07-15)

	var (
		expPop float64
		expVa  float64
		expVc  float64
	)

	switch releaseGroup {
	case "BHDStudio":
		expPop = 0.2440
		expVa = 0.0770
		expVc = 0.3280
	case "FraMeSToR":
		expPop = 0.0250
		expVa = 0.2980
		expVc = 0.6900
	}

	logMaxPop := math.Log(maxPop)
	logMaxVc := math.Log(maxVc)

	popNorm := math.Log(popularity+1.0) / logMaxPop
	vaNorm := voteAverage / maxVa
	vcNorm := math.Log(float64(voteCount)+1.0) / logMaxVc

	// Guard against zero/negative after normalization
	if popNorm < 0 || vaNorm < 0 || vcNorm < 0 {
		return 0.0
	}

	score := math.Pow(popNorm, expPop) * math.Pow(vaNorm, expVa) * math.Pow(vcNorm, expVc)

	// Recency Boost for new movies lacking historical vote_count
	if year > newReleaseBaseYear {
		yearsNew := year - newReleaseBaseYear
		recencyBoost := 1.0 + (float64(yearsNew) * newReleaseBoost)
		score *= recencyBoost
	}

	// Language modifier
	if language == "en" {
		score *= enBoost
	}

	// Genre modifiers
	for _, genre := range genres {
		if genre == "History" {
			score *= historyBoost
		}
		if genre == "Horror" {
			score *= horrorPenalty
		}
	}

	return score
}
