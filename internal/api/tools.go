package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/agentauth"
	"github.com/chinmay/switchboard/internal/ai"
	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/tenant"
)

// Tools is the agent-facing API. It is separate from the browser API: callers
// authenticate with a per-call token, and every mutation goes through the same
// booking store the rest of the product uses.
type Tools struct {
	Jobs        *jobs.Registry
	Calls       *calls.Store
	AI          *ai.Client
	Tenants     *tenant.Registry
	AgentKey    string
	TokenSecret string
}

type agentSessionRequest struct {
	TenantID   string `json:"tenant_id"`
	CallID     string `json:"call_id"`
	Direction  string `json:"direction"`
	FromNumber string `json:"from_number"`
	ToNumber   string `json:"to_number"`
	RoomName   string `json:"room_name"`
}

type agentSessionResponse struct {
	Token  string `json:"token"`
	CallID string `json:"call_id"`
	Tenant struct {
		ID            string         `json:"id"`
		Name          string         `json:"name"`
		Services      []string       `json:"services"`
		Windows       []string       `json:"windows"`
		Persona       tenant.Persona `json:"persona"`
		Disclosure    string         `json:"disclosure"`
		ManagerNumber string         `json:"manager_number,omitempty"`
	} `json:"tenant"`
}

// AgentSession exchanges the agent bootstrap key for a short-lived per-call
// token and returns the tenant configuration the agent needs.
func (t *Tools) AgentSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !agentauth.BootstrapKey(r.Header.Get("X-Agent-Key"), t.AgentKey) {
			http.Error(w, "invalid agent key", http.StatusUnauthorized)
			return
		}
		var body agentSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid session", http.StatusBadRequest)
			return
		}
		body.TenantID = strings.TrimSpace(body.TenantID)
		body.CallID = strings.TrimSpace(body.CallID)
		if body.TenantID == "" || body.CallID == "" {
			http.Error(w, "tenant_id and call_id are required", http.StatusBadRequest)
			return
		}
		cfg, ok := t.Tenants.Get(body.TenantID)
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		if err := t.Calls.Start(r.Context(), calls.Call{
			ID:           body.CallID,
			TenantID:     body.TenantID,
			Direction:    firstNonEmpty(body.Direction, "inbound"),
			FromNumber:   body.FromNumber,
			ToNumber:     body.ToNumber,
			RoomName:     body.RoomName,
			ManagerPhone: cfg.ManagerPhone(),
		}); err != nil {
			http.Error(w, "could not start call record", http.StatusInternalServerError)
			return
		}
		token, err := agentauth.Mint(t.TokenSecret, agentauth.Claims{
			TenantID: body.TenantID,
			CallID:   body.CallID,
		})
		if err != nil {
			http.Error(w, "could not mint agent token", http.StatusInternalServerError)
			return
		}
		resp := agentSessionResponse{Token: token, CallID: body.CallID}
		resp.Tenant.ID = cfg.ID
		resp.Tenant.Name = cfg.Name
		resp.Tenant.Services = cfg.Services
		resp.Tenant.Windows = cfg.Windows
		resp.Tenant.Persona = cfg.Persona
		resp.Tenant.ManagerNumber = cfg.ManagerPhone()
		resp.Tenant.Disclosure = disclosure(cfg)
		writeJSON(w, resp)
	}
}

func disclosure(cfg tenant.Tenant) string {
	if strings.TrimSpace(cfg.Disclosure) != "" {
		return cfg.Disclosure
	}
	return "Hi! This is an AI assistant for " + cfg.Name +
		". This call can be recorded for quality and service purposes. " +
		"Are you comfortable with the call being recorded, or would you prefer to continue without recording?"
}

func (t *Tools) Availability() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Service string `json:"service"`
			Date    string `json:"date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch that.")
			return
		}
		if strings.TrimSpace(body.Service) == "" || strings.TrimSpace(body.Date) == "" {
			toolError(w, "invalid_request", "I need a service and a date.")
			return
		}
		store := t.Jobs.For(claims.TenantID)
		windows, err := store.AvailabilityContext(r.Context(), body.Service, body.Date)
		if err != nil {
			toolError(w, "schedule_unavailable", "I'm having trouble checking the schedule.")
			return
		}
		alternatives, err := store.AlternativesContext(r.Context(), body.Service, body.Date, 7)
		if err != nil {
			toolError(w, "schedule_unavailable", "I'm having trouble checking the schedule.")
			return
		}
		writeJSON(w, map[string]any{
			"ok":           true,
			"date":         body.Date,
			"windows":      windows,
			"alternatives": alternatives,
		})
	}
}

type bookingConfirmation struct {
	TenantID string `json:"tenant_id"`
	CallID   string `json:"call_id"`
	Service  string `json:"service"`
	Address  string `json:"address"`
	Date     string `json:"date"`
	Window   string `json:"window"`
	Phone    string `json:"phone"`
}

// SaveAddress persists a validated address on the call. Booking reads it from
// the draft, so the model cannot substitute a different address later.
func (t *Tools) SaveAddress() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch that address.")
			return
		}
		address := strings.Join(strings.Fields(body.Address), " ")
		if len([]rune(address)) < 6 {
			toolError(w, "invalid_address", "Could you repeat the full address, including a landmark?")
			return
		}
		if err := t.Calls.SaveDraftField(r.Context(), claims.TenantID, claims.CallID, "address", address); err != nil {
			toolError(w, "store_unavailable", "I can't save that address right now.")
			return
		}
		writeJSON(w, map[string]any{
			"ok":        true,
			"address":   address,
			"read_back": "I have " + address + ". Is that right?",
		})
	}
}

// SavePhone validates and persists the callback number. Booking requires this
// draft field, which prevents the model from inventing a number.
func (t *Tools) SavePhone() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Phone string `json:"phone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch that number.")
			return
		}
		phone, ok := validIndianPhone(body.Phone)
		if !ok {
			toolError(w, "invalid_phone", "I need a 10-digit mobile number starting with 6, 7, 8, or 9.")
			return
		}
		if err := t.Calls.SaveDraftField(r.Context(), claims.TenantID, claims.CallID, "phone", phone); err != nil {
			toolError(w, "store_unavailable", "I can't save that number right now.")
			return
		}
		writeJSON(w, map[string]any{
			"ok":        true,
			"phone":     phone,
			"read_back": spokenPhone(phone) + ". Is that right?",
		})
	}
}

// ConfirmBooking validates and canonicalizes the details, then returns a
// signed token. The agent must read the returned text back to the caller and
// only then call Book with the token, which prevents booking half-heard data.
func (t *Tools) ConfirmBooking() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Service string `json:"service"`
			Date    string `json:"date"`
			Window  string `json:"window"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch those details.")
			return
		}
		cfg, ok := t.Tenants.Get(claims.TenantID)
		if !ok {
			toolError(w, "unknown_tenant", "That business is not configured.")
			return
		}
		service, ok := tenant.CanonicalService(cfg.Services, body.Service)
		if !ok {
			toolError(w, "unknown_service", "I can book "+strings.Join(cfg.Services, ", ")+".")
			return
		}
		draft, err := t.Calls.Draft(r.Context(), claims.CallID)
		if err != nil {
			toolError(w, "store_unavailable", "I can't check those details right now.")
			return
		}
		if draft.Address == "" {
			toolError(w, "invalid_address", "I need the service address before I can book.")
			return
		}
		if draft.Phone == "" {
			toolError(w, "invalid_phone", "I need the callback number before I can book.")
			return
		}
		address := draft.Address
		date, err := time.Parse("2006-01-02", body.Date)
		if err != nil {
			toolError(w, "invalid_date", "What day works for you?")
			return
		}
		today := time.Now().UTC().Truncate(24 * time.Hour)
		if date.Before(today) {
			toolError(w, "invalid_date", "That date has passed. What day works for you?")
			return
		}
		window := strings.ToLower(strings.TrimSpace(body.Window))
		if !jobs.ValidWindow(window) {
			toolError(w, "invalid_window", "Which window: morning, afternoon, or evening?")
			return
		}
		phone := draft.Phone
		confirmation := bookingConfirmation{
			TenantID: claims.TenantID,
			CallID:   claims.CallID,
			Service:  service,
			Address:  address,
			Date:     body.Date,
			Window:   window,
			Phone:    phone,
		}
		token, err := agentauth.Seal(t.TokenSecret, confirmation, 10*time.Minute)
		if err != nil {
			toolError(w, "store_unavailable", "I can't prepare that booking right now.")
			return
		}
		writeJSON(w, map[string]any{
			"ok":              true,
			"confirmation_id": token,
			"service":         service,
			"address":         address,
			"date":            body.Date,
			"window":          window,
			"phone":           phone,
			"read_back": fmt.Sprintf(
				"%s at %s on %s, %s window, callback %s. Is that correct?",
				service, address, prettyDate(date), window, spokenPhone(phone),
			),
		})
	}
}

func (t *Tools) Book() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			ConfirmationID string `json:"confirmation_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ConfirmationID) == "" {
			toolError(w, "not_confirmed", "Let me read the details back before I book.")
			return
		}
		var confirmation bookingConfirmation
		if err := agentauth.Open(t.TokenSecret, body.ConfirmationID, &confirmation); err != nil {
			toolError(w, "not_confirmed", "Let me read the details back again before I book.")
			return
		}
		if confirmation.TenantID != claims.TenantID || confirmation.CallID != claims.CallID {
			toolError(w, "not_confirmed", "Let me read the details back again before I book.")
			return
		}
		request := jobs.BookingRequest{
			Service: confirmation.Service,
			Date:    confirmation.Date,
			Window:  confirmation.Window,
			Address: confirmation.Address,
			Phone:   confirmation.Phone,
		}
		request.IdempotencyKey = jobs.BookingIdempotencyKey(claims.CallID+"/book", request)
		job, err := t.Jobs.For(claims.TenantID).BookContext(r.Context(), request)
		if err != nil {
			switch {
			case errors.Is(err, jobs.ErrNoStaff), errors.Is(err, jobs.ErrNoAvailability):
				toolError(w, "no_availability", "That slot is no longer available.")
			case errors.Is(err, jobs.ErrInvalid):
				toolError(w, "invalid_request", "Some booking details are missing.")
			default:
				toolError(w, "store_unavailable", "I can't complete the booking right now.")
			}
			return
		}
		writeJSON(w, map[string]any{"ok": true, "job": job})
	}
}

func validIndianPhone(value string) (string, bool) {
	digits := jobs.NormalizePhone(value)
	if len(digits) == 12 && strings.HasPrefix(digits, "91") {
		digits = digits[2:]
	}
	if len(digits) != 10 {
		return "", false
	}
	switch digits[0] {
	case '6', '7', '8', '9':
		return digits, true
	default:
		return "", false
	}
}

func spokenPhone(digits string) string {
	if len(digits) != 10 {
		return digits
	}
	return "+91 " + digits[:5] + " " + digits[5:]
}

func prettyDate(date time.Time) string {
	return date.Format("Monday 2 January")
}

func (t *Tools) Reschedule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Phone  string `json:"phone"`
			Date   string `json:"date"`
			Window string `json:"window"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch that.")
			return
		}
		store := t.Jobs.For(claims.TenantID)
		found, err := store.ByPhoneContext(r.Context(), body.Phone)
		if err != nil {
			toolError(w, "store_unavailable", "I can't look up that booking right now.")
			return
		}
		if len(found) == 0 {
			toolError(w, "not_found", "I couldn't find a booking for that number.")
			return
		}
		if err := store.RescheduleContext(r.Context(), found[0].ID, body.Date, body.Window); err != nil {
			switch {
			case errors.Is(err, jobs.ErrNoStaff), errors.Is(err, jobs.ErrNoAvailability):
				toolError(w, "no_availability", "That new slot is no longer available.")
			case errors.Is(err, jobs.ErrNotFound):
				toolError(w, "not_found", "I couldn't find that booking.")
			default:
				toolError(w, "store_unavailable", "I can't update that booking right now.")
			}
			return
		}
		writeJSON(w, map[string]any{"ok": true, "job_id": found[0].ID})
	}
}

func (t *Tools) Cancel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Phone string `json:"phone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch that.")
			return
		}
		store := t.Jobs.For(claims.TenantID)
		found, err := store.ByPhoneContext(r.Context(), body.Phone)
		if err != nil {
			toolError(w, "store_unavailable", "I can't look up that booking right now.")
			return
		}
		if len(found) == 0 {
			toolError(w, "not_found", "I couldn't find a booking for that number.")
			return
		}
		if err := store.CancelContext(r.Context(), found[0].ID); err != nil {
			toolError(w, "store_unavailable", "I can't cancel that booking right now.")
			return
		}
		writeJSON(w, map[string]any{"ok": true, "job_id": found[0].ID})
	}
}

func (t *Tools) Lookup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Phone string `json:"phone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch that.")
			return
		}
		found, err := t.Jobs.For(claims.TenantID).ByPhoneContext(r.Context(), body.Phone)
		if err != nil {
			toolError(w, "store_unavailable", "I can't look up that booking right now.")
			return
		}
		writeJSON(w, map[string]any{"ok": true, "jobs": found})
	}
}

func (t *Tools) Policy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Query string `json:"query"`
			Lang  string `json:"lang"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Query) == "" {
			toolError(w, "invalid_request", "I need a question to look up.")
			return
		}
		if t.AI == nil {
			toolError(w, "ungrounded", "I'm not able to look that up right now.")
			return
		}
		lang := body.Lang
		if lang == "" {
			lang = "en"
		}
		answer, err := t.AI.Answer(r.Context(), claims.TenantID, body.Query, 3, lang)
		if err != nil || !answer.Grounded {
			toolError(w, "ungrounded", "I'm not sure about that.")
			return
		}
		writeJSON(w, map[string]any{
			"ok":        true,
			"answer":    answer.Answer,
			"citations": answer.Citations,
		})
	}
}

func (t *Tools) Consent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Allowed bool `json:"allowed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch that.")
			return
		}
		if err := t.Calls.SetConsent(r.Context(), claims.CallID, body.Allowed); err != nil {
			toolError(w, "store_unavailable", "I can't save that right now.")
			return
		}
		writeJSON(w, map[string]any{"ok": true, "allowed": body.Allowed})
	}
}

func (t *Tools) Escalate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "I didn't catch that.")
			return
		}
		if strings.TrimSpace(body.Reason) == "" {
			body.Reason = "caller_requested_human"
		}
		if err := t.Calls.Escalate(r.Context(), claims.CallID, body.Reason); err != nil {
			toolError(w, "store_unavailable", "I can't update that call right now.")
			return
		}
		manager := ""
		if cfg, ok := t.Tenants.Get(claims.TenantID); ok {
			manager = cfg.ManagerPhone()
		}
		writeJSON(w, map[string]any{
			"ok":             true,
			"manager_number": manager,
			"reason":         body.Reason,
		})
	}
}

func (t *Tools) End() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := agentauth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Outcome string `json:"outcome"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			toolError(w, "invalid_request", "invalid outcome")
			return
		}
		if err := t.Calls.End(r.Context(), claims.CallID, body.Outcome); err != nil {
			toolError(w, "store_unavailable", "I can't update that call right now.")
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func toolError(w http.ResponseWriter, code, message string) {
	writeJSON(w, map[string]any{"ok": false, "code": code, "message": message})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
