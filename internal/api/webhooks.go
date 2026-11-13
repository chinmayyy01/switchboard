package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

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
		case "egress_started", "egress_updated", "egress_ended":
			if event.Event == "egress_ended" && len(event.EgressInfo) > 0 {
				var info egressInfo
				if err := json.Unmarshal(event.EgressInfo, &info); err != nil {
					slog.Warn("could not parse egress info", "err", err)
					break
				}
				status := egressStatus(info.Status)
				key := ""
				var durationMS int64
				if info.File != nil {
					key = info.File.Filename
					durationMS = info.File.Duration / int64(time.Millisecond)
				}
				if durationMS == 0 && info.EndedAt > 0 && info.StartedAt > 0 {
					durationMS = (info.EndedAt - info.StartedAt) / int64(time.Millisecond)
				}
				n, err := store.FinishRecordingByEgress(r.Context(), info.EgressID, status, key, info.Error, durationMS)
				if err != nil {
					slog.Error("finish recording", "egress", info.EgressID, "err", err)
				} else if n > 0 {
					slog.Info("recording finished", "egress", info.EgressID, "status", status, "has_key", key != "")
				}
			}
		default:
			slog.Debug("livekit webhook ignored", "event", event.Event, "room", room)
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

type egressInfo struct {
	EgressID  string `json:"egressId"`
	RoomName  string `json:"roomName"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	StartedAt int64  `json:"startedAt"`
	EndedAt   int64  `json:"endedAt"`
	File      *struct {
		Filename string `json:"filename"`
		Duration int64  `json:"duration"`
		Size     int64  `json:"size"`
	} `json:"file"`
}

func egressStatus(status string) string {
	switch strings.ToUpper(status) {
	case "EGRESS_COMPLETE":
		return "completed"
	case "EGRESS_FAILED":
		return "failed"
	case "EGRESS_ABORTED":
		return "aborted"
	default:
		return "failed"
	}
}
