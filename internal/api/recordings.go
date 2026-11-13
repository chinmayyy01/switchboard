package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/agentauth"
	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/storage"
)

// RecordingStarted is called by the agent after Egress accepts the job. The
// recording row is only created when the call has recording consent.
func (t *Tools) RecordingStarted() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			EgressID string `json:"egress_id"`
			Channels int    `json:"channels"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.EgressID) == "" {
			toolError(w, "invalid_request", "missing egress id")
			return
		}
		recorded, err := t.Calls.StartRecording(r.Context(), calls.Recording{
			CallID:   claims.CallID,
			TenantID: claims.TenantID,
			EgressID: strings.TrimSpace(body.EgressID),
			Channels: body.Channels,
		})
		if err != nil {
			toolError(w, "store_unavailable", "I can't save the recording state right now.")
			return
		}
		writeJSON(w, map[string]any{"ok": true, "recorded": recorded})
	}
}

// RecordingStopped is the agent's best-effort stop signal. The egress_ended
// webhook remains authoritative for status and the object key.
func (t *Tools) RecordingStopped() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			EgressID string `json:"egress_id"`
			Error    string `json:"error"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "invalid recording state")
			return
		}
		status := "completed"
		if strings.TrimSpace(body.Error) != "" {
			status = "failed"
		}
		n, err := t.Calls.FinishRecordingByCall(r.Context(), claims.CallID, status, body.Error)
		if err != nil {
			toolError(w, "store_unavailable", "I can't save the recording state right now.")
			return
		}
		writeJSON(w, map[string]any{"ok": true, "updated": n})
	}
}

type RecordingAPI struct {
	Calls   *calls.Store
	Storage *storage.S3
}

// Get returns a short-lived presigned URL. The bucket stays private.
func (a *RecordingAPI) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		rec, err := a.Calls.Recording(r.Context(), r.PathValue("id"), "")
		if err != nil || rec == nil {
			http.Error(w, "recording not found", http.StatusNotFound)
			return
		}
		if !canSeeTenant(account, rec.TenantID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if rec.Status != "completed" || rec.ObjectKey == "" {
			http.Error(w, "recording is not available", http.StatusConflict)
			return
		}
		if a.Storage == nil || !a.Storage.Configured() {
			http.Error(w, "recording storage is not configured", http.StatusServiceUnavailable)
			return
		}
		url, err := a.Storage.PresignGet(r.Context(), rec.ObjectKey, 10*time.Minute)
		if err != nil {
			http.Error(w, "could not sign recording", http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]any{
			"url":        url,
			"expires_in": 600,
			"status":     rec.Status,
		})
	}
}

// Delete removes the stored object and marks the row deleted. Owner-only per
// ADR 0001; the audit trail keeps the call and consent evidence.
func (a *RecordingAPI) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !account.IsOwner {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		rec, err := a.Calls.Recording(r.Context(), r.PathValue("id"), "")
		if err != nil || rec == nil {
			http.Error(w, "recording not found", http.StatusNotFound)
			return
		}
		if rec.ObjectKey != "" {
			if a.Storage == nil || !a.Storage.Configured() {
				http.Error(w, "recording storage is not configured", http.StatusServiceUnavailable)
				return
			}
			if err := a.Storage.Delete(r.Context(), rec.ObjectKey); err != nil {
				http.Error(w, "could not delete recording", http.StatusBadGateway)
				return
			}
		}
		if err := a.Calls.MarkRecordingDeleted(r.Context(), rec.CallID); err != nil {
			http.Error(w, "could not update recording", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}
