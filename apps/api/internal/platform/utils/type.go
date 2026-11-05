// Package utils provides small, dependency-free helpers shared by services.
package utils

import "strings"

// SplitTrimmedCommaList splits input on commas, trims whitespace from every
// entry, and omits empty entries. It returns nil when input contains no
// non-empty entries.
func SplitTrimmedCommaList(input string) []string {
	if input == "" {
		return nil
	}

	var values []string

	for value := range strings.SplitSeq(input, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}

	return values
}
