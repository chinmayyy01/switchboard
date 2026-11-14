package calls

import "testing"

func TestNormalizeCallerNumber(t *testing.T) {
	cases := []struct {
		input string
		e164  string
	}{
		{"9876543210", "+919876543210"},
		{"+91 98765 43210", "+919876543210"},
		{"09876543210", "+919876543210"},
		{"91-98765-43210", "+919876543210"},
		{"+1 (510) 555-0100", "+15105550100"},
		{"  +44 20 7946 0958  ", "+442079460958"},
		{"unknown", "unknown"},
		{"", ""},
	}
	for _, c := range cases {
		got, original := NormalizeCallerNumber(c.input)
		if got != c.e164 {
			t.Fatalf("NormalizeCallerNumber(%q) = %q, want %q", c.input, got, c.e164)
		}
		if original != c.input && c.input != "  +44 20 7946 0958  " {
			t.Fatalf("original = %q for %q", original, c.input)
		}
	}
}
