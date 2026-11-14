package tenant

import (
	"strings"
	"unicode"
)

// CanonicalService maps a misheard or informal service phrase to one of the
// tenant's configured services. Speech-to-text commonly confuses similar
// sounding pairs ("train cleaning" vs "drain cleaning"), so an exact match is
// too strict for a phone line.
func CanonicalService(services []string, spoken string) (string, bool) {
	normalized := normalizePhrase(spoken)
	if normalized == "" {
		return "", false
	}
	best := ""
	bestDistance := 1 << 30
	for _, service := range services {
		candidate := normalizePhrase(service)
		if candidate == "" {
			continue
		}
		if normalized == candidate || strings.Contains(normalized, candidate) || strings.Contains(candidate, normalized) {
			return service, true
		}
		distance := levenshtein(normalized, candidate)
		threshold := len([]rune(candidate)) / 4
		if threshold < 2 {
			threshold = 2
		}
		if distance <= threshold && distance < bestDistance {
			best, bestDistance = service, distance
		}
	}
	if best != "" {
		return best, true
	}
	// Fall back to a distinctive word shared with a configured service, so
	// "my drain is blocked" still resolves to "drain cleaning".
	spokenTokens := strings.Fields(normalized)
	bestScore := 0
	for _, service := range services {
		score := 0
		for _, token := range strings.Fields(normalizePhrase(service)) {
			if len(token) < 4 {
				continue
			}
			for _, spokenToken := range spokenTokens {
				if spokenToken == token {
					score++
					break
				}
			}
		}
		if score > bestScore {
			best, bestScore = service, score
		}
	}
	if best != "" {
		return best, true
	}
	return "", false
}

func normalizePhrase(value string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastSpace = false
		case unicode.IsSpace(r) || r == '-' || r == ',' || r == '.':
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := 0; j <= len(br); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

func min(values ...int) int {
	out := values[0]
	for _, value := range values[1:] {
		if value < out {
			out = value
		}
	}
	return out
}
