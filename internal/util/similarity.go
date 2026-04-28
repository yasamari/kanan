package util

import "github.com/agnivade/levenshtein"

func Similarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	dist := levenshtein.ComputeDistance(s1, s2)
	maxLen := len([]rune(s1))
	if l2 := len([]rune(s2)); l2 > maxLen {
		maxLen = l2
	}

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(dist)/float64(maxLen)
}
