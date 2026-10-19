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
	droppedFrames = promauto.NewCounter(prometheus.CounterOpts{
		Name: "switchboard_dropped_frames_total", Help: "Audio frames dropped under backpressure.",
	})
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
func Dropped(n int) {
	if n > 0 {
		droppedFrames.Add(float64(n))
	}
}

func ObserveStage(stage string, d time.Duration) {
	stageLatency.WithLabelValues(stage).Observe(d.Seconds())
}
func ObserveTurn(d time.Duration) { turnLatency.Observe(d.Seconds()) }

func Handler() http.Handler { return promhttp.Handler() }
