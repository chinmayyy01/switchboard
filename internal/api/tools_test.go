package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chinmay/switchboard/internal/agentauth"
	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/staff"
	"github.com/chinmay/switchboard/internal/tenant"
)

func TestAgentToolsBookOnceAndRecordConsent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := "api-test-rivera"
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM call_drafts WHERE tenant_id = $1", tenantID)
		db.ExecContext(context.Background(), "DELETE FROM calls WHERE tenant_id = $1", tenantID)
		db.ExecContext(context.Background(), "DELETE FROM jobs WHERE tenant_id = $1", tenantID)
		db.ExecContext(context.Background(), "DELETE FROM employees WHERE tenant_id = $1", tenantID)
		db.Close()
	})
	registry, err := jobs.NewPostgresRegistry(ctx, db, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	store := registry.For(tenantID)
	store.SetWindows([]string{"morning", "afternoon"})
	if _, err := store.SaveEmployeeContext(ctx, staff.Employee{
		ID: "tech-1", Name: "Maya Patel", Role: staff.RoleTechnician,
		Skills: []string{"leak repair", "drain cleaning"}, Days: []string{"mon"},
		Shifts: []string{"morning"}, Active: true,
	}); err != nil {
		t.Fatal(err)
	}

	tools := &Tools{
		Jobs:        registry,
		Calls:       calls.New(db),
		Tenants:     tenant.New(tenant.Tenant{ID: tenantID, Name: "Test Rivera", Services: []string{"leak repair", "drain cleaning"}, Windows: []string{"morning", "afternoon"}}),
		AgentKey:    "bootstrap-key",
		TokenSecret: "token-secret",
	}

	sessionBody, _ := json.Marshal(map[string]string{
		"tenant_id": tenantID,
		"call_id":   "call-test-1",
		"room_name": "call-test-1",
	})
	sessionReq := httptest.NewRequest(http.MethodPost, "/internal/agent/session", bytes.NewReader(sessionBody))
	sessionReq.Header.Set("X-Agent-Key", "bootstrap-key")
	sessionRec := httptest.NewRecorder()
	tools.AgentSession()(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusOK {
		t.Fatalf("session status=%d body=%s", sessionRec.Code, sessionRec.Body.String())
	}
	var session struct {
		Token  string `json:"token"`
		CallID string `json:"call_id"`
	}
	if err := json.Unmarshal(sessionRec.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.Token == "" || session.CallID != "call-test-1" {
		t.Fatalf("session response = %+v", session)
	}

	nextMonday := time.Now().UTC().AddDate(0, 0, 1)
	for nextMonday.Weekday() != time.Monday {
		nextMonday = nextMonday.AddDate(0, 0, 1)
	}
	addressBody, _ := json.Marshal(map[string]string{
		"address": "Flat 4B, Palm Grove, 12 Main Street, near the temple",
	})
	if got := toolRequest(t, tools.SaveAddress(), "token-secret", session.Token, addressBody); got.Code != http.StatusOK {
		t.Fatalf("save address status=%d body=%s", got.Code, got.Body.String())
	}
	phoneBody, _ := json.Marshal(map[string]string{"phone": "+91 98765 43210"})
	if got := toolRequest(t, tools.SavePhone(), "token-secret", session.Token, phoneBody); got.Code != http.StatusOK {
		t.Fatalf("save phone status=%d body=%s", got.Code, got.Body.String())
	}
	details, _ := json.Marshal(map[string]string{
		"service": "train cleaning",
		"date":    nextMonday.Format("2006-01-02"),
		"window":  "morning",
	})
	confirm := toolRequest(t, tools.ConfirmBooking(), "token-secret", session.Token, details)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm status=%d body=%s", confirm.Code, confirm.Body.String())
	}
	var confirmed struct {
		OK             bool   `json:"ok"`
		ConfirmationID string `json:"confirmation_id"`
		Service        string `json:"service"`
		Phone          string `json:"phone"`
		ReadBack       string `json:"read_back"`
	}
	if err := json.Unmarshal(confirm.Body.Bytes(), &confirmed); err != nil {
		t.Fatal(err)
	}
	if !confirmed.OK || confirmed.ConfirmationID == "" {
		t.Fatalf("confirmation = %+v body=%s", confirmed, confirm.Body.String())
	}
	if confirmed.Service != "drain cleaning" {
		t.Fatalf("misheard service was not canonicalized: %q", confirmed.Service)
	}
	if confirmed.Phone != "9876543210" {
		t.Fatalf("phone not normalized: %q", confirmed.Phone)
	}
	if confirmed.ReadBack == "" {
		t.Fatal("missing read-back")
	}

	withoutToken := toolRequest(t, tools.Book(), "token-secret", session.Token, []byte(`{}`))
	var refused struct {
		OK   bool   `json:"ok"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(withoutToken.Body.Bytes(), &refused); err != nil {
		t.Fatal(err)
	}
	if refused.OK || refused.Code != "not_confirmed" {
		t.Fatalf("booking without confirmation was allowed: %+v", refused)
	}

	bookBody, _ := json.Marshal(map[string]string{"confirmation_id": confirmed.ConfirmationID})
	book := toolRequest(t, tools.Book(), "token-secret", session.Token, bookBody)
	if book.Code != http.StatusOK {
		t.Fatalf("book status=%d body=%s", book.Code, book.Body.String())
	}
	var booked struct {
		OK  bool     `json:"ok"`
		Job jobs.Job `json:"job"`
	}
	if err := json.Unmarshal(book.Body.Bytes(), &booked); err != nil {
		t.Fatal(err)
	}
	if !booked.OK || booked.Job.EmployeeID != "tech-1" {
		t.Fatalf("booking = %+v body=%s", booked, book.Body.String())
	}
	if booked.Job.Service != "drain cleaning" {
		t.Fatalf("booked service = %q", booked.Job.Service)
	}

	// The same call retrying the same confirmation must not create a second visit.
	again := toolRequest(t, tools.Book(), "token-secret", session.Token, bookBody)
	var bookedAgain struct {
		OK  bool     `json:"ok"`
		Job jobs.Job `json:"job"`
	}
	if err := json.Unmarshal(again.Body.Bytes(), &bookedAgain); err != nil {
		t.Fatal(err)
	}
	if bookedAgain.Job.ID != booked.Job.ID {
		t.Fatalf("retry created a new job: %s vs %s", booked.Job.ID, bookedAgain.Job.ID)
	}

	// A second call with no saved phone must not be confirmable.
	noPhoneSession := toolRequest(t, tools.Book(), "token-secret", session.Token, []byte(`{"confirmation_id":"x"}`))
	if noPhoneSession.Code != http.StatusOK {
		t.Fatalf("unexpected status for invalid token: %d", noPhoneSession.Code)
	}

	consentBody := []byte(`{"allowed":false}`)
	consent := toolRequest(t, tools.Consent(), "token-secret", session.Token, consentBody)
	if consent.Code != http.StatusOK {
		t.Fatalf("consent status=%d body=%s", consent.Code, consent.Body.String())
	}
	allowed, err := calls.New(db).Consent(ctx, session.CallID)
	if err != nil || allowed == nil || *allowed {
		t.Fatalf("consent not persisted as false: %v %v", allowed, err)
	}

	unauthorized := toolRequest(t, tools.Book(), "wrong-secret", session.Token, bookBody)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status=%d", unauthorized.Code)
	}
}

func toolRequest(t *testing.T, handler http.HandlerFunc, secret, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/internal/tools", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	agentauth.Require(secret, handler)(recorder, request)
	return recorder
}

func TestSpokenPhoneUsesIndianFormat(t *testing.T) {
	if got := spokenPhone("6202416257"); got != "+91 62024 16257" {
		t.Fatalf("spokenPhone = %q", got)
	}
}

func TestValidIndianPhoneAcceptsCommonFormats(t *testing.T) {
	cases := map[string]string{
		"6202416257":      "6202416257",
		"(620) 241-6257":  "6202416257",
		"+91 62024 16257": "6202416257",
		"91-62024-16257":  "6202416257",
	}
	for input, want := range cases {
		got, ok := validIndianPhone(input)
		if !ok || got != want {
			t.Fatalf("%q -> %q (%v), want %q", input, got, ok, want)
		}
	}
	if _, ok := validIndianPhone("12345"); ok {
		t.Fatal("short number accepted")
	}
}
