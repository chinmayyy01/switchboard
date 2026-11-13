package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func signWebhook(t *testing.T, apiKey, secret string, body []byte, exp int64) string {
	t.Helper()
	sum := sha256.Sum256(body)
	claims := map[string]any{
		"iss":    apiKey,
		"sha256": base64.StdEncoding.EncodeToString(sum[:]),
		"exp":    exp,
	}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return input + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhook(t *testing.T) {
	body := []byte(`{"event":"participant_joined","room":{"name":"call-1"},"participant":{"identity":"caller","kind":"SIP","attributes":{"sip.phoneNumber":"+919876543210"}}}`)
	token := signWebhook(t, "key", "secret", body, time.Now().Add(time.Minute).Unix())

	event, err := VerifyWebhook("key", "secret", body, token)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if event.Event != "participant_joined" || event.Room == nil || event.Room.Name != "call-1" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if !event.Participant.IsSIP() {
		t.Fatal("SIP participant was not detected")
	}
	if got := event.Participant.Attributes["sip.phoneNumber"]; got != "+919876543210" {
		t.Fatalf("phone = %q", got)
	}
}

func TestVerifyWebhookRejectsTamperedBody(t *testing.T) {
	body := []byte(`{"event":"room_finished","room":{"name":"call-1"}}`)
	token := signWebhook(t, "key", "secret", body, time.Now().Add(time.Minute).Unix())
	if _, err := VerifyWebhook("key", "secret", append(body, ' '), token); err == nil {
		t.Fatal("tampered body was accepted")
	}
}

func TestVerifyWebhookRejectsWrongKeyOrExpiry(t *testing.T) {
	body := []byte(`{"event":"room_finished","room":{"name":"call-1"}}`)
	token := signWebhook(t, "key", "secret", body, time.Now().Add(time.Minute).Unix())
	if _, err := VerifyWebhook("other", "secret", body, token); err == nil {
		t.Fatal("wrong issuer was accepted")
	}
	expired := signWebhook(t, "key", "secret", body, time.Now().Add(-time.Minute).Unix())
	if _, err := VerifyWebhook("key", "secret", body, expired); err == nil {
		t.Fatal("expired webhook was accepted")
	}
}
