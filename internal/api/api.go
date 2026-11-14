package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/eval"
	"github.com/chinmay/switchboard/internal/livekit"
)

func Calls(s *calls.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tenant := requestedTenant(account, r)
		records, err := s.List(r.Context(), tenant, 200)
		if err != nil {
			http.Error(w, "could not load calls", http.StatusInternalServerError)
			return
		}
		writeJSON(w, records)
	}
}

func LiveCalls(s *calls.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tenant := requestedTenant(account, r)
		records, err := s.Live(r.Context(), tenant)
		if err != nil {
			http.Error(w, "could not load live calls", http.StatusInternalServerError)
			return
		}
		writeJSON(w, records)
	}
}

func Label(s *calls.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		rec, err := s.CallByID(r.Context(), id)
		if err != nil {
			http.Error(w, "unknown call", http.StatusNotFound)
			return
		}
		if !canSeeTenant(account, rec.Tenant) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body struct {
			ShouldEscalate bool `json:"should_escalate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if err := s.SetLabel(r.Context(), id, body.ShouldEscalate); err != nil {
			if errors.Is(err, calls.ErrNotFound) {
				http.Error(w, "unknown call", http.StatusNotFound)
				return
			}
			http.Error(w, "could not save label", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func Report(s *calls.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tenant := requestedTenant(account, r)
		labeled, err := s.Labeled(r.Context(), tenant)
		if err != nil {
			http.Error(w, "could not load report", http.StatusInternalServerError)
			return
		}
		preds := make([]eval.Prediction, 0, len(labeled))
		for _, item := range labeled {
			preds = append(preds, eval.Prediction{
				Name:      item.ID,
				Predicted: item.Predicted,
				Actual:    item.Actual,
			})
		}
		writeJSON(w, eval.Score(preds))
	}
}

// SupervisorToken lets a signed-in supervisor join the live room to listen and
// speak. It never dispatches another agent.
func SupervisorToken(s *calls.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		rec, err := s.CallByID(r.Context(), r.PathValue("id"))
		if err != nil {
			http.Error(w, "unknown call", http.StatusNotFound)
			return
		}
		if !canSeeTenant(account, rec.Tenant) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if rec.RoomName == "" {
			http.Error(w, "call has no room", http.StatusConflict)
			return
		}
		if account.Role == auth.RoleViewer {
			http.Error(w, "owner or manager role required", http.StatusForbidden)
			return
		}
		apiKey := os.Getenv("LIVEKIT_API_KEY")
		apiSecret := os.Getenv("LIVEKIT_API_SECRET")
		url := os.Getenv("LIVEKIT_URL")
		if apiKey == "" || apiSecret == "" || url == "" {
			http.Error(w, "LiveKit is not configured", http.StatusServiceUnavailable)
			return
		}
		token, err := livekit.Mint(livekit.Options{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Identity:  "supervisor-" + account.ID,
			Name:      account.Email,
			TTL:       2 * time.Hour,
			Grants: livekit.Grants{
				Room:           rec.RoomName,
				RoomJoin:       true,
				CanPublish:     true,
				CanSubscribe:   true,
				CanPublishData: true,
			},
		})
		if err != nil {
			http.Error(w, "could not create supervisor token", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"url":   url,
			"token": token,
			"room":  rec.RoomName,
		})
	}
}

func requestedTenant(account auth.Account, r *http.Request) string {
	if !account.IsOwner {
		return account.TenantID
	}
	return strings.TrimSpace(r.URL.Query().Get("tenant"))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
