package main

import "strings"

// sanitizeTitle performs tiered post-flag.Parse cleanup of the -title
// value. flag.Set assigns verbatim, so literal quote pollution from
// callers survives into os.Args. The tiers:
//
//  1. Strip one matched pair of surrounding double quotes ("S&X" -> S&X).
//  2. Unescape literal backslash-escaped quote pairs (\" -> "), which may
//     expose a new surrounding pair that is then stripped.
//  3. Leave everything else intact.
//
// Legitimate inner quotes (e.g. Alice "Wonderland") and apostrophes are
// preserved. Idempotent for clean titles and single-level caller pollution;
// pathological inputs of four or more consecutive quote characters reach
// their fixed point on a second pass. Harmless in practice: main() applies
// the sanitizer exactly once per invocation.
func sanitizeTitle(title string) string {
	s := title

	if len(s) >= 2 && strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = s[1 : len(s)-1]
	}

	if strings.Contains(s, "\\\"") {
		s = strings.ReplaceAll(s, "\\\"", "\"")
		if len(s) >= 2 && strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
			s = s[1 : len(s)-1]
		}
	}

	return s
}
