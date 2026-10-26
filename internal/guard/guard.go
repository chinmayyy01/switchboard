package guard

import (
	"regexp"
	"strings"
)

var injectionPatterns = []string{
	"ignore previous instructions",
	"ignore all previous",
	"ignore the previous",
	"system prompt",
	"you are now",
	"jailbreak",
	"disregard your instructions",
	"reveal your instructions",
	"print your prompt",
}

func LooksLikeInjection(text string) bool {
	low := strings.ToLower(text)
	for _, p := range injectionPatterns {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	cardRe  = regexp.MustCompile(`\b\d(?:[ -]?\d){12,18}\b`)
)

func Redact(text string) string {
	text = emailRe.ReplaceAllString(text, "[email]")
	text = cardRe.ReplaceAllString(text, "[number]")
	return text
}
