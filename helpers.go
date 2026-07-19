package main

import "strings"

// preprocessArgs merges a bare (unquoted) -title value spread across
// multiple argv tokens into a single token so flag.Parse() handles it.
// e.g. ["-title", "The", "Dark", "Knight", "-year", "2008"]
//
//	-> ["-title", "The Dark Knight", "-year", "2008"]
func preprocessArgs(args []string) []string {
	// Flags that consume a value. We stop collecting title words when we
	// hit one of these.
	valueFlags := map[string]bool{
		"-title":          true,
		"--title":         true,
		"-year":           true,
		"--year":          true,
		"-release-group":  true,
		"--release-group": true,
	}

	var result []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "-title" || arg == "--title" {
			result = append(result, arg)
			i++
			var titleParts []string
			for i < len(args) && !valueFlags[args[i]] {
				titleParts = append(titleParts, args[i])
				i++
			}
			if len(titleParts) > 0 {
				result = append(result, strings.Join(titleParts, " "))
			}
			continue
		}
		result = append(result, arg)
		i++
	}
	return result
}
