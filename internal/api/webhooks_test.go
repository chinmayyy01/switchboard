package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/jobs"
)

func signWebhookBody(t *testing.T, key, secret string, body []byte) string {
	t.Helper()
	sum := sha256.Sum256(body)
	claims := map[string]any{
		"iss":    key,
		"sha256": base64.StdEncoding.EncodeToString(sum[:]),
		"exp":    time.Now().Add(time.Minute).Unix(),
	}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return input + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestLiveKitWebhookStoresCallerAndClosesCall(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := "webhook-test-" + time.Now().Format("150405.000000")
	callID := tenantID + "-call"
	room := tenantID + "-room"
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM calls WHERE tenant_id = $1", tenantID)
		db.Close()
	})
	store := calls.New(db)
	if err := store.Start(ctx, calls.Call{ID: callID, TenantID: tenantID, RoomName: room}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LIVEKIT_API_KEY", "key")
	t.Setenv("LIVEKIT_API_SECRET", "secret")

	post := func(event string, body string) {
		t.Helper()
		payload := []byte(body)
		req := httptest.NewRequest(http.MethodPost, "/internal/livekit/webhook", bytes.NewReader(payload))
		req.Header.Set("Authorization", signWebhookBody(t, "key", "secret", payload))
		rec := httptest.NewRecorder()
		LiveKitWebhook(store)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", event, rec.Code, rec.Body.String())
		}
	}

	post("participant_joined", `{
		"event":"participant_joined",
		"room":{"name":"`+room+`"},
		"participant":{"attributes":{
			"sip.phoneNumber":"+919876543210",
			"sip.trunkPhoneNumber":"+911140000000",
			"sip.callID":"sip-1"
		}}
	}`)
	var from, to string
	if err := db.QueryRowContext(ctx,
		"SELECT from_number, to_number FROM calls WHERE id = $1", callID,
	).Scan(&from, &to); err != nil {
		t.Fatal(err)
	}
	if from != "+919876543210" || to != "+911140000000" {
		t.Fatalf("caller id from=%q to=%q", from, to)
	}

	post("room_finished", `{"event":"room_finished","room":{"name":"`+room+`"}}`)
	var outcome string
	var ended bool
	if err := db.QueryRowContext(ctx,
		"SELECT outcome, ended_at IS NOT NULL FROM calls WHERE id = $1", callID,
	).Scan(&outcome, &ended); err != nil {
		t.Fatal(err)
	}
	if outcome != "completed" || !ended {
		t.Fatalf("outcome=%q ended=%v", outcome, ended)
	}
}
