package main

import "testing"

// Table-driven tests for sanitizeTitle: tiered post-flag.Parse cleanup of
// the -title value. Strips one matched pair of surrounding double quotes,
// then unescapes literal backslash-escaped quote pairs (\"). Legitimate
// inner quotes and apostrophes are preserved; the function is idempotent.
func TestSanitizeTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain title unchanged", "S&X", "S&X"},
		{"surrounding quotes stripped", "\"S&X\"", "S&X"},
		{"escaped wrapper unescaped and stripped", "\\\"S&X\\\"", "S&X"},
		{"apostrophe preserved", "Grey's Anatomy", "Grey's Anatomy"},
		{"inner quotes preserved", "Alice \"Wonderland\"", "Alice \"Wonderland\""},
		{"escaped inner quotes become plain quotes", "Alice \\\"Wonderland\\\"", "Alice \"Wonderland\""},
		{"unmatched leading quote left intact", "\"Dune", "\"Dune"},
		{"unmatched trailing quote left intact", "Dune\"", "Dune\""},
		{"single quote character left intact", "\"", "\""},
		{"empty string unchanged", "", ""},
		{"quoted empty becomes empty", "\"\"", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeTitle(tt.input)
			if got != tt.want {
				t.Fatalf("sanitizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
			again := sanitizeTitle(got)
			if again != tt.want {
				t.Fatalf("sanitizeTitle not idempotent: sanitizeTitle(%q) = %q, want %q", got, again, tt.want)
			}
		})
	}
}
