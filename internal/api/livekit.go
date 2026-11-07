package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/livekit"
	"github.com/chinmay/switchboard/internal/tenant"
)

// LiveKitToken mints a browser room token for console test calls. The agent is
// dispatched to the room by the same tenant metadata used on inbound SIP calls.
func LiveKitToken(tenants *tenant.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		apiKey := os.Getenv("LIVEKIT_API_KEY")
		apiSecret := os.Getenv("LIVEKIT_API_SECRET")
		url := os.Getenv("LIVEKIT_URL")
		if apiKey == "" || apiSecret == "" || url == "" {
			http.Error(w, "LiveKit is not configured", http.StatusServiceUnavailable)
			return
		}
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant"))
		if account.IsOwner && tenantID == "" {
			tenantID = "rivera"
		}
		if !account.IsOwner {
			tenantID = account.TenantID
		}
		cfg, ok := tenants.Get(tenantID)
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		room := strings.TrimSpace(r.URL.Query().Get("room"))
		if room == "" {
			room = "console-" + account.ID + "-" + time.Now().Format("150405")
		}
		metadata, _ := json.Marshal(map[string]string{
			"tenant_id":  cfg.ID,
			"call_id":    "browser-" + room,
			"direction":  "browser",
			"account_id": account.ID,
		})
		agentName := os.Getenv("LIVEKIT_AGENT_NAME")
		if agentName == "" {
			agentName = "switchboard-receptionist"
		}
		token, err := livekit.Mint(livekit.Options{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Identity:  account.ID,
			Name:      account.Email,
			Metadata:  string(metadata),
			TTL:       30 * time.Minute,
			Grants: livekit.Grants{
				Room:           room,
				RoomJoin:       true,
				CanPublish:     true,
				CanSubscribe:   true,
				CanPublishData: true,
			},
			RoomConfig: &livekit.RoomConfiguration{
				Agents: []livekit.RoomAgentDispatch{{
					AgentName: agentName,
					Metadata:  string(metadata),
				}},
			},
		})
		if err != nil {
			http.Error(w, "could not create room token", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"url":       url,
			"token":     token,
			"room":      room,
			"tenant_id": cfg.ID,
		})
	}
}
