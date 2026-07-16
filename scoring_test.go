package main

import (
	"math"
	"testing"
)

const tolerance = 1e-9

// referenceScore is an independent reimplementation of the additive scoring
// formula used to verify parity with the production ComputeScore.
func referenceScore(
	popularity, voteAverage float64,
	voteCount, year int64,
	language, releaseGroup string,
	genres []string,
) (float64, bool) {
	weights := map[string]struct {
		wVc, wPop, wVa, wEn, wYear float64
	}{
		"BHDStudio": {2.29, 1.35, 0.53, 0.40, 0.06},
		"FraMeSToR": {2.49, -0.54, 1.52, 0.20, 0.09},
	}

	w, ok := weights[releaseGroup]
	if !ok {
		return 0.0, false
	}

	logMaxPop := math.Log1p(3000.0)
	logMaxVc := math.Log1p(150000.0)

	popNorm := math.Max(math.Log1p(popularity)/logMaxPop, 1e-9)
	vaNorm := math.Max(voteAverage/10.0, 1e-9)
	vcNorm := math.Max(math.Log1p(float64(voteCount))/logMaxVc, 1e-9)

	score := w.wVc*vcNorm + w.wPop*popNorm + w.wVa*vaNorm

	if year > 2020 {
		score += w.wYear * float64(year-2020)
	}

	if language == "en" {
		score += w.wEn
	}

	genreSet := make(map[string]struct{}, len(genres))
	for _, g := range genres {
		genreSet[g] = struct{}{}
	}
	if _, ok := genreSet["Horror"]; ok {
		score += -0.10
	}
	if _, ok := genreSet["History"]; ok {
		score += 0.05
	}
	if _, ok := genreSet["Family"]; ok {
		score += 0.05
	}

	return score, true
}

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

func TestGetConfig(t *testing.T) {
	t.Run("valid groups", func(t *testing.T) {
		for _, group := range []string{"BHDStudio", "FraMeSToR"} {
			cfg, err := getConfig(group)
			if err != nil {
				t.Fatalf("getConfig(%q) unexpected error: %v", group, err)
			}
			if cfg.scoreThreshold == 0 {
				t.Fatalf("getConfig(%q) returned zero threshold", group)
			}
		}
	})

	t.Run("unsupported group", func(t *testing.T) {
		_, err := getConfig("UnknownGroup")
		if err == nil {
			t.Fatal("getConfig(\"UnknownGroup\") expected error, got nil")
		}
	})
}

func TestComputeScore_ParityWithReference(t *testing.T) {
	cases := []struct {
		name         string
		popularity   float64
		voteAverage  float64
		voteCount    int64
		year         int64
		language     string
		releaseGroup string
		genres       []string
		expectZero   bool // true when release group is invalid
	}{
		{
			name:         "BHDStudio basic no modifiers",
			popularity:   100,
			voteAverage:  7.0,
			voteCount:    5000,
			year:         2019,
			language:     "es",
			releaseGroup: "BHDStudio",
			genres:       []string{"Drama"},
		},
		{
			name:         "FraMeSToR basic no modifiers",
			popularity:   200,
			voteAverage:  8.0,
			voteCount:    10000,
			year:         2018,
			language:     "fr",
			releaseGroup: "FraMeSToR",
			genres:       []string{"Action"},
		},
		{
			name:         "BHDStudio with recency boost",
			popularity:   50,
			voteAverage:  6.5,
			voteCount:    2000,
			year:         2023,
			language:     "de",
			releaseGroup: "BHDStudio",
			genres:       []string{"Comedy"},
		},
		{
			name:         "BHDStudio with english modifier",
			popularity:   80,
			voteAverage:  7.5,
			voteCount:    8000,
			year:         2019,
			language:     "en",
			releaseGroup: "BHDStudio",
			genres:       []string{"Thriller"},
		},
		{
			name:         "FraMeSToR with horror penalty",
			popularity:   300,
			voteAverage:  6.0,
			voteCount:    15000,
			year:         2021,
			language:     "en",
			releaseGroup: "FraMeSToR",
			genres:       []string{"Horror", "Thriller"},
		},
		{
			name:         "BHDStudio with history and family boosts",
			popularity:   150,
			voteAverage:  8.5,
			voteCount:    20000,
			year:         2020,
			language:     "en",
			releaseGroup: "BHDStudio",
			genres:       []string{"History", "Family", "Drama"},
		},
		{
			name:         "all modifiers combined",
			popularity:   1000,
			voteAverage:  9.0,
			voteCount:    50000,
			year:         2024,
			language:     "en",
			releaseGroup: "BHDStudio",
			genres:       []string{"Horror", "History", "Family"},
		},
		{
			name:         "duplicate genres applied once",
			popularity:   200,
			voteAverage:  7.0,
			voteCount:    10000,
			year:         2022,
			language:     "en",
			releaseGroup: "BHDStudio",
			genres:       []string{"Horror", "Horror", "Horror"},
		},
		{
			name:         "zero vote count",
			popularity:   500,
			voteAverage:  8.0,
			voteCount:    0,
			year:         2023,
			language:     "en",
			releaseGroup: "BHDStudio",
			genres:       []string{},
		},
		{
			name:         "zero popularity",
			popularity:   0,
			voteAverage:  7.0,
			voteCount:    1000,
			year:         2019,
			language:     "es",
			releaseGroup: "FraMeSToR",
			genres:       []string{"Drama"},
		},
		{
			name:         "all zeros",
			popularity:   0,
			voteAverage:  0,
			voteCount:    0,
			year:         2019,
			language:     "es",
			releaseGroup: "BHDStudio",
			genres:       []string{},
		},
		{
			name:         "FraMeSToR negative pop weight with high pop",
			popularity:   2000,
			voteAverage:  5.0,
			voteCount:    50000,
			year:         2025,
			language:     "en",
			releaseGroup: "FraMeSToR",
			genres:       []string{"Action"},
		},
		{
			name:         "year exactly 2020 no recency",
			popularity:   100,
			voteAverage:  7.0,
			voteCount:    5000,
			year:         2020,
			language:     "es",
			releaseGroup: "BHDStudio",
			genres:       []string{},
		},
		{
			name:         "invalid release group returns zero",
			popularity:   100,
			voteAverage:  7.0,
			voteCount:    5000,
			year:         2023,
			language:     "en",
			releaseGroup: "Unknown",
			genres:       []string{"Action"},
			expectZero:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeScore(
				tc.popularity, tc.voteAverage,
				tc.voteCount, tc.year,
				tc.language, tc.releaseGroup,
				tc.genres,
			)

			if tc.expectZero {
				if got != 0.0 {
					t.Fatalf("expected 0.0 for invalid group, got %f", got)
				}
				return
			}

			ref, ok := referenceScore(
				tc.popularity, tc.voteAverage,
				tc.voteCount, tc.year,
				tc.language, tc.releaseGroup,
				tc.genres,
			)
			if !ok {
				t.Fatal("referenceScore failed for valid group")
			}

			if !almostEqual(got, ref, tolerance) {
				t.Fatalf("parity mismatch: ComputeScore=%f  reference=%f  diff=%e",
					got, ref, math.Abs(got-ref))
			}
		})
	}
}

func TestComputeScore_DuplicateGenresAppliedOnce(t *testing.T) {
	single := ComputeScore(200, 7.0, 10000, 2022, "en", "BHDStudio", []string{"Horror"})
	triple := ComputeScore(200, 7.0, 10000, 2022, "en", "BHDStudio", []string{"Horror", "Horror", "Horror"})
	if !almostEqual(single, triple, tolerance) {
		t.Fatalf("duplicate genres should apply once: single=%f triple=%f", single, triple)
	}
}

func TestComputeScore_ScoreThresholds(t *testing.T) {
	// Verify that a clearly desirable movie scores above its threshold
	// and a clearly undesirable movie scores below.
	for _, tc := range []struct {
		name         string
		releaseGroup string
		popularity   float64
		voteAverage  float64
		voteCount    int64
		year         int64
		language     string
		genres       []string
		wantAbove    bool
	}{
		{
			name:         "BHDStudio desirable",
			releaseGroup: "BHDStudio",
			popularity:   500,
			voteAverage:  8.0,
			voteCount:    20000,
			year:         2023,
			language:     "en",
			genres:       []string{"Action"},
			wantAbove:    true,
		},
		{
			name:         "BHDStudio undesirable",
			releaseGroup: "BHDStudio",
			popularity:   1,
			voteAverage:  3.0,
			voteCount:    10,
			year:         2015,
			language:     "xx",
			genres:       []string{"Horror"},
			wantAbove:    false,
		},
		{
			name:         "FraMeSToR desirable",
			releaseGroup: "FraMeSToR",
			popularity:   800,
			voteAverage:  8.5,
			voteCount:    50000,
			year:         2024,
			language:     "en",
			genres:       []string{"Drama"},
			wantAbove:    true,
		},
		{
			name:         "FraMeSToR undesirable",
			releaseGroup: "FraMeSToR",
			popularity:   5,
			voteAverage:  2.0,
			voteCount:    50,
			year:         2010,
			language:     "xx",
			genres:       []string{"Horror"},
			wantAbove:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := getConfig(tc.releaseGroup)
			score := ComputeScore(
				tc.popularity, tc.voteAverage,
				tc.voteCount, tc.year,
				tc.language, tc.releaseGroup,
				tc.genres,
			)
			above := score >= cfg.scoreThreshold
			if above != tc.wantAbove {
				t.Fatalf("score=%f threshold=%f wantAbove=%v gotAbove=%v",
					score, cfg.scoreThreshold, tc.wantAbove, above)
			}
		})
	}
}
