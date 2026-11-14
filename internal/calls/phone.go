package calls

import "strings"

// NormalizeCallerNumber returns an E.164 best-effort value and the original
// trimmed input. Indian mobiles are the primary case; unknown formats are
// preserved rather than rejected, because a caller ID we cannot normalize is
// still better than no caller ID.
func NormalizeCallerNumber(raw string) (e164 string, original string) {
	original = strings.TrimSpace(raw)
	if original == "" {
		return "", ""
	}

	var digits strings.Builder
	for _, r := range original {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if d == "" {
		return original, original
	}

	switch {
	case len(d) == 10 && d[0] >= '6' && d[0] <= '9':
		return "+91" + d, original
	case len(d) == 11 && d[0] == '0' && d[1] >= '6' && d[1] <= '9':
		return "+91" + d[1:], original
	case len(d) == 12 && strings.HasPrefix(d, "91") && d[2] >= '6' && d[2] <= '9':
		return "+" + d, original
	case strings.HasPrefix(original, "+") && len(d) >= 8:
		return "+" + d, original
	default:
		return d, original
	}
}
