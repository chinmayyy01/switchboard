package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/chinmay/switchboard/internal/agentauth"
	"github.com/chinmay/switchboard/internal/metrics"
)

// AgentMetrics accepts small, low-cardinality metric events from the Python
// worker. The Go process owns the Prometheus registry, so the agent never
// needs its own scrape endpoint.
func (t *Tools) AgentMetrics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := agentauth.FromContext(r.Context()); !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Event    string `json:"event"`
			Stage    string `json:"stage"`
			Tool     string `json:"tool"`
			Code     string `json:"code"`
			Provider string `json:"provider"`
			Reason   string `json:"reason"`
			MS       int    `json:"ms"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid metric", http.StatusBadRequest)
			return
		}
		switch strings.ToLower(strings.TrimSpace(body.Event)) {
		case "turn":
			metrics.ObserveAgentTurn(body.Stage, body.MS)
		case "tool_error":
			metrics.ToolError(body.Tool, body.Code)
		case "fallback":
			metrics.Fallback(body.Stage, body.Provider)
		case "recording_failure":
			metrics.RecordingFailure()
		default:
			http.Error(w, "unknown metric", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}
