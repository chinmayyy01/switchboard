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

func TestEgressEndedWebhookFinishesRecording(t *testing.T) {
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
	if err := store.SetConsent(ctx, callID, true, true, "en"); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.StartRecording(ctx, calls.Recording{
		CallID: callID, TenantID: tenantID, EgressID: "egress-1",
	}); err != nil || !ok {
		t.Fatalf("start recording ok=%v err=%v", ok, err)
	}

	t.Setenv("LIVEKIT_API_KEY", "key")
	t.Setenv("LIVEKIT_API_SECRET", "secret")
	body := []byte(`{"event":"egress_ended","room":{"name":"` + room + `"},"egressInfo":{"egressId":"egress-1","roomName":"` + room + `","status":"EGRESS_COMPLETE","file":{"filename":"recordings/x.ogg","duration":5000000000}}}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/livekit/webhook", bytes.NewReader(body))
	req.Header.Set("Authorization", signWebhookBody(t, "key", "secret", body))
	rec := httptest.NewRecorder()
	LiveKitWebhook(store)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, err := store.Recording(ctx, callID, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" || got.ObjectKey != "recordings/x.ogg" || got.DurationMS != 5000 {
		t.Fatalf("recording = %+v", got)
	}
}
