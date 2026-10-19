package escalate

type Signal struct {
	Turns              int     `json:"turns"`
	LowConfidenceTurns int     `json:"low_confidence_turns"`
	RepeatedIntent     int     `json:"repeated_intent"`
	Sentiment          float64 `json:"sentiment"`
	AskedForHuman      bool    `json:"asked_for_human"`
	Emergency          bool    `json:"emergency"`
	OutOfScope         bool    `json:"out_of_scope"`
}

type Decision struct {
	Escalate bool   `json:"escalate"`
	Reason   string `json:"reason"`
}

type Policy interface {
	Decide(Signal) Decision
}

type RulePolicy struct {
	MinConfidence    float64
	MaxLowConfStreak int
	MaxTurns         int
	MaxRepeats       int
	MinSentiment     float64
}

func DefaultRulePolicy() RulePolicy {
	return RulePolicy{
		MinConfidence:    0.5,
		MaxLowConfStreak: 3,
		MaxTurns:         12,
		MaxRepeats:       3,
		MinSentiment:     -0.6,
	}
}

func (p RulePolicy) Decide(s Signal) Decision {
	switch {
	case s.Emergency:
		return Decision{true, "emergency"}
	case s.AskedForHuman:
		return Decision{true, "caller_asked_for_human"}
	case s.OutOfScope:
		return Decision{true, "out_of_scope"}
	case s.RepeatedIntent >= p.MaxRepeats:
		return Decision{true, "repeated_misunderstanding"}
	case s.LowConfidenceTurns >= p.MaxLowConfStreak:
		return Decision{true, "low_asr_confidence"}
	case s.Sentiment <= p.MinSentiment:
		return Decision{true, "negative_sentiment"}
	case s.Turns >= p.MaxTurns:
		return Decision{true, "max_turns"}
	}
	return Decision{false, "contained"}
}
