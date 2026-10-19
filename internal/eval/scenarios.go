package eval

import "github.com/chinmay/switchboard/internal/escalate"

func Scenarios() []Scenario {
	return []Scenario{

		{Name: "gas_smell", Signal: escalate.Signal{Turns: 2, Emergency: true}, ShouldEscalate: true},
		{Name: "burst_pipe", Signal: escalate.Signal{Turns: 2, Emergency: true}, ShouldEscalate: true},
		{Name: "no_heat_winter", Signal: escalate.Signal{Turns: 3, Emergency: true}, ShouldEscalate: true},
		{Name: "asks_for_human", Signal: escalate.Signal{Turns: 2, AskedForHuman: true}, ShouldEscalate: true},
		{Name: "repeated_misunderstanding", Signal: escalate.Signal{Turns: 5, RepeatedIntent: 3}, ShouldEscalate: true},
		{Name: "low_asr_streak", Signal: escalate.Signal{Turns: 4, LowConfidenceTurns: 4}, ShouldEscalate: true},
		{Name: "angry_caller", Signal: escalate.Signal{Turns: 3, Sentiment: -0.8}, ShouldEscalate: true},
		{Name: "warranty_claim", Signal: escalate.Signal{Turns: 2, OutOfScope: true}, ShouldEscalate: true},
		{Name: "refund_request", Signal: escalate.Signal{Turns: 3, OutOfScope: true}, ShouldEscalate: true},
		{Name: "too_many_turns", Signal: escalate.Signal{Turns: 15}, ShouldEscalate: true},

		{Name: "routine_booking", Signal: escalate.Signal{Turns: 4}, ShouldEscalate: false},
		{Name: "reschedule", Signal: escalate.Signal{Turns: 3}, ShouldEscalate: false},
		{Name: "cancel", Signal: escalate.Signal{Turns: 3}, ShouldEscalate: false},
		{Name: "hours_question", Signal: escalate.Signal{Turns: 1}, ShouldEscalate: false},
		{Name: "service_area", Signal: escalate.Signal{Turns: 2}, ShouldEscalate: false},
		{Name: "price_question", Signal: escalate.Signal{Turns: 2}, ShouldEscalate: false},
		{Name: "one_low_confidence_turn", Signal: escalate.Signal{Turns: 4, LowConfidenceTurns: 1}, ShouldEscalate: false},
		{Name: "mildly_negative", Signal: escalate.Signal{Turns: 4, Sentiment: -0.3}, ShouldEscalate: false},
		{Name: "one_repeat", Signal: escalate.Signal{Turns: 4, RepeatedIntent: 1}, ShouldEscalate: false},
		{Name: "normal_eight_turns", Signal: escalate.Signal{Turns: 8}, ShouldEscalate: false},

		{Name: "gas_word_false_trigger", Signal: escalate.Signal{Turns: 2, Emergency: true}, ShouldEscalate: false},
		{Name: "rude_tech_missed", Signal: escalate.Signal{Turns: 3, Sentiment: -0.5}, ShouldEscalate: true},
	}
}
