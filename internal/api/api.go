package api

import (
	"encoding/json"
	"net/http"

	"github.com/chinmay/switchboard/internal/eval"
	"github.com/chinmay/switchboard/internal/hub"
	"github.com/chinmay/switchboard/internal/store"
)

func Calls(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant := r.URL.Query().Get("tenant")
		records := s.List()
		if tenant != "" {
			filtered := records[:0]
			for _, rec := range records {
				if rec.Tenant == tenant {
					filtered = append(filtered, rec)
				}
			}
			records = filtered
		}
		writeJSON(w, records)
	}
}

func Label(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			ShouldEscalate bool `json:"should_escalate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if !s.SetLabel(id, body.ShouldEscalate) {
			http.Error(w, "unknown call", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func Report(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant := r.URL.Query().Get("tenant")
		labels := s.Labels()
		var preds []eval.Prediction
		for _, rec := range s.List() {
			if tenant != "" && rec.Tenant != tenant {
				continue
			}
			label, ok := labels[rec.ID]
			if !ok {
				continue
			}
			preds = append(preds, eval.Prediction{
				Name:      rec.ID,
				Predicted: rec.Escalated,
				Actual:    label,
			})
		}
		writeJSON(w, eval.Score(preds))
	}
}

func TakeOver(h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !h.TakeOver(id) {
			http.Error(w, "unknown call", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
