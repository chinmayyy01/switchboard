package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var buckets = []float64{.05, .1, .25, .5, .75, 1, 1.5, 2, 3, 5, 8}

var (
	activeCalls = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "switchboard_active_calls", Help: "Live call sessions.",
	})
	callsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_calls_total", Help: "Calls started.",
	})
	escalationsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_escalations_total", Help: "Escalations triggered.",
	})
	bookingsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_bookings_total", Help: "Calls that completed a booking.",
	})
	llmPromptTokens = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_llm_prompt_tokens_total", Help: "LLM prompt tokens.",
	})
	llmCompletionTokens = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_llm_completion_tokens_total", Help: "LLM completion tokens.",
	})
	droppedFrames = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_dropped_frames_total", Help: "Audio frames dropped under backpressure.",
	})
	reapedCalls = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_reaped_calls_total", Help: "Calls closed by the stale-call reaper.",
	})
	transcriptTurns = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_transcript_turns_total", Help: "Transcript turns persisted after consent.",
	})
	agentTurnLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "switchboard_agent_turn_latency_seconds", Help: "Agent-reported turn latency by stage.", Buckets: buckets,
	}, []string{"stage"})
	agentToolErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "switchboard_agent_tool_errors_total", Help: "Agent tool errors by tool and code.",
	}, []string{"tool", "code"})
	agentFallbacks = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "switchboard_agent_fallbacks_total", Help: "Provider fallbacks by stage and provider.",
	}, []string{"stage", "provider"})
	agentBootstrapFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_agent_bootstrap_failures_total", Help: "Agent sessions that could not bootstrap.",
	})
	recordingFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_recording_failures_total", Help: "Recording start or storage failures.",
	})
	billingUnpriced = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_billing_unpriced_total", Help: "Usage records with no price table entry.",
	})
	livekitActiveCalls = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "switchboard_livekit_active_calls", Help: "LiveKit call sessions that have bootstrapped.",
	})
	livekitCallsStarted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "switchboard_livekit_calls_started_total", Help: "LiveKit calls started by source.",
	}, []string{"source"})
	livekitCallsEnded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "switchboard_livekit_calls_ended_total", Help: "LiveKit calls ended by outcome.",
	}, []string{"outcome"})
	stageLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "switchboard_stage_latency_seconds", Help: "Per-stage turn latency.", Buckets: buckets,
	}, []string{"stage"})
	turnLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "switchboard_turn_latency_seconds", Help: "End-to-end turn latency.", Buckets: buckets,
	})
)

func CallStarted() { callsTotal.Inc(); activeCalls.Inc() }
func CallEnded()   { activeCalls.Dec() }
func Escalation()  { escalationsTotal.Inc() }
func Booking()     { bookingsTotal.Inc() }
func ObserveTokens(prompt, completion int) {
	if prompt > 0 {
		llmPromptTokens.Add(float64(prompt))
	}
	if completion > 0 {
		llmCompletionTokens.Add(float64(completion))
	}
}
func Dropped(n int) {
	if n > 0 {
		droppedFrames.Add(float64(n))
	}
}
func CallsReaped(n int64) {
	if n > 0 {
		reapedCalls.Add(float64(n))
	}
}
func TranscriptTurn(recorded bool) {
	if recorded {
		transcriptTurns.Inc()
	}
}
func ObserveAgentTurn(stage string, ms int) {
	if stage != "" && ms >= 0 {
		agentTurnLatency.WithLabelValues(stage).Observe(float64(ms) / 1000)
	}
}
func ToolError(tool, code string) {
	if tool == "" {
		tool = "unknown"
	}
	if code == "" {
		code = "unknown"
	}
	agentToolErrors.WithLabelValues(tool, code).Inc()
}
func Fallback(stage, provider string) {
	if stage == "" {
		stage = "unknown"
	}
	if provider == "" {
		provider = "unknown"
	}
	agentFallbacks.WithLabelValues(stage, provider).Inc()
}
func BootstrapFailure() { agentBootstrapFailures.Inc() }
func RecordingFailure() { recordingFailures.Inc() }
func BillingUnpriced()  { billingUnpriced.Inc() }
func LiveKitCallStarted(source string) {
	if source == "" {
		source = "unknown"
	}
	livekitCallsStarted.WithLabelValues(source).Inc()
	livekitActiveCalls.Inc()
}
func LiveKitCallEnded(outcome string) {
	if outcome == "" {
		outcome = "unknown"
	}
	livekitCallsEnded.WithLabelValues(outcome).Inc()
	livekitActiveCalls.Dec()
}

func ObserveStage(stage string, d time.Duration) {
	stageLatency.WithLabelValues(stage).Observe(d.Seconds())
}
func ObserveTurn(d time.Duration) { turnLatency.Observe(d.Seconds()) }

func Handler() http.Handler { return promhttp.Handler() }
