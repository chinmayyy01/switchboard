package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/livekit"
	"github.com/chinmay/switchboard/internal/metrics"
)

const maxWebhookBody = 1 << 20

// LiveKitWebhook is the authoritative call lifecycle signal. It is
// intentionally outside the agent-token middleware: LiveKit authenticates with
// a signed JWT over the raw body instead.
func LiveKitWebhook(store *calls.Store) http.HandlerFunc {
	apiKey := os.Getenv("LIVEKIT_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_API_SECRET")
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
		if err != nil {
			http.Error(w, "could not read body", http.StatusBadRequest)
			return
		}
		event, err := livekit.VerifyWebhook(apiKey, apiSecret, body, r.Header.Get("Authorization"))
		if err != nil {
			if errors.Is(err, livekit.ErrWebhookNotConfigured) {
				slog.Error("livekit webhook received but LIVEKIT_API_KEY/SECRET are not set")
				http.Error(w, "webhook not configured", http.StatusServiceUnavailable)
				return
			}
			slog.Warn("rejected livekit webhook", "err", err)
			http.Error(w, "invalid webhook", http.StatusUnauthorized)
			return
		}

		room := ""
		if event.Room != nil {
			room = event.Room.Name
		}
		switch event.Event {
		case "participant_joined":
			if event.Participant != nil && room != "" {
				attrs := event.Participant.Attributes
				from := attrs["sip.phoneNumber"]
				to := attrs["sip.trunkPhoneNumber"]
				sipCallID := attrs["sip.callID"]
				if from != "" || to != "" {
					n, err := store.SetCallerByRoom(r.Context(), room, from, to, sipCallID)
					if err != nil {
						slog.Error("store caller id", "room", room, "err", err)
					} else if n > 0 {
						slog.Info("caller id stored", "room", room, "has_from", from != "", "has_to", to != "")
					}
				}
			}
		case "room_finished":
			if room != "" {
				n, err := store.EndByRoom(r.Context(), room, "completed", "room_finished")
				if err != nil {
					slog.Error("close call from webhook", "room", room, "err", err)
				} else if n > 0 {
					metrics.LiveKitCallEnded("room_finished")
					slog.Info("call closed by webhook", "room", room, "event", event.Event)
				}
			}
		default:
			slog.Debug("livekit webhook ignored", "event", event.Event, "room", room)
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}
