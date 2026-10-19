package escalate

import "strings"

var (
	emergencyWords  = []string{"gas", "flood", "burst", "no heat", "no water", "sewage", "carbon monoxide", "emergency", "electrical hazard", "smell"}
	humanWords      = []string{"human", "representative", "real person", "talk to someone", "speak to someone", "manager", "supervisor"}
	outOfScopeWords = []string{"warranty", "refund", "bill", "billing", "invoice", "lawyer", "lawsuit", "insurance", "complaint"}
	negWords        = []string{"angry", "terrible", "awful", "useless", "furious", "unacceptable", "ridiculous", "worst", "rude", "disappointed"}
	posWords        = []string{"thanks", "great", "good", "appreciate", "perfect", "fine"}
)

type Tracker struct {
	turns      int
	lastText   string
	repeats    int
	lowStreak  int
	emergency  bool
	askedHuman bool
	outOfScope bool
	sentiment  float64
}

func NewTracker() *Tracker { return &Tracker{} }

func (t *Tracker) Observe(text string, confidence float64) Signal {
	t.turns++

	if text != "" && text == t.lastText {
		t.repeats++
	} else {
		t.repeats = 0
	}
	t.lastText = text

	if confidence < 0.5 {
		t.lowStreak++
	} else {
		t.lowStreak = 0
	}

	lower := strings.ToLower(text)
	if containsAny(lower, emergencyWords) {
		t.emergency = true
	}
	if containsAny(lower, humanWords) {
		t.askedHuman = true
	}
	if containsAny(lower, outOfScopeWords) {
		t.outOfScope = true
	}
	t.sentiment = sentiment(lower)

	return t.Signal()
}

func (t *Tracker) Signal() Signal {
	return Signal{
		Turns:              t.turns,
		LowConfidenceTurns: t.lowStreak,
		RepeatedIntent:     t.repeats,
		Sentiment:          t.sentiment,
		AskedForHuman:      t.askedHuman,
		Emergency:          t.emergency,
		OutOfScope:         t.outOfScope,
	}
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func sentiment(s string) float64 {
	pos, neg := 0, 0
	for _, w := range posWords {
		if strings.Contains(s, w) {
			pos++
		}
	}
	for _, w := range negWords {
		if strings.Contains(s, w) {
			neg++
		}
	}
	if pos+neg == 0 {
		return 0
	}
	return float64(pos-neg) / float64(pos+neg)
}
