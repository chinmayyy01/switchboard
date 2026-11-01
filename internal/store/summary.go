package store

import "strings"

func Summarize(turns []Turn, escalated bool, reason string) string {
	if escalated {
		if reason != "" {
			return "Escalated to a human: " + reason + "."
		}
		return "Escalated to a human."
	}

	first := ""
	for _, t := range turns {
		if t.Role == "caller" && strings.TrimSpace(t.Text) != "" {
			first = t.Text
			break
		}
	}
	if first == "" {
		return "Handled automatically."
	}
	return "Handled automatically. Caller asked: " + clip(first, 120)
}

func clip(s string, n int) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
