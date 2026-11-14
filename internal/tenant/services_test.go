package tenant

import "testing"

func TestCanonicalServiceHandlesMisheardPhrases(t *testing.T) {
	services := []string{"leak repair", "drain cleaning", "water heater", "ac service"}
	cases := map[string]string{
		"train cleaning":      "drain cleaning",
		"drain clean":         "drain cleaning",
		"leak repair":         "leak repair",
		"water heater":        "water heater",
		"ac service":          "ac service",
		"my drain is blocked": "drain cleaning",
	}
	for spoken, want := range cases {
		got, ok := CanonicalService(services, spoken)
		if !ok || got != want {
			t.Fatalf("%q -> %q (%v), want %q", spoken, got, ok, want)
		}
	}
	if _, ok := CanonicalService(services, "install a pool"); ok {
		t.Fatal("unrelated service matched")
	}
}
