package core

import (
	"strings"
	"unicode"
)

func NormalizeChampion(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func FuzzyScore(query, candidate string) int {
	q, c := []rune(NormalizeChampion(query)), []rune(NormalizeChampion(candidate))
	if len(q) == 0 {
		return 0
	}
	if len(c) == 0 {
		return -1
	}
	if string(q) == string(c) {
		return 1000
	}
	if strings.HasPrefix(string(c), string(q)) {
		return 800 - len(c)
	}
	if i := strings.Index(string(c), string(q)); i >= 0 {
		return 600 - i - len(c)
	}
	qi, last, score := 0, -2, 300
	for ci, r := range c {
		if r != q[qi] {
			continue
		}
		if ci == last+1 {
			score += 12
		} else {
			score -= ci - last - 1
		}
		last = ci
		qi++
		if qi == len(q) {
			return score - len(c)
		}
	}
	return -1
}
