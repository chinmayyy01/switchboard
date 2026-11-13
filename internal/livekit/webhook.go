package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidWebhook       = errors.New("invalid livekit webhook")
	ErrWebhookNotConfigured = errors.New("livekit webhook is not configured")
)

// WebhookEvent is the subset of LiveKit's WebhookEvent payload this service
// needs. LiveKit encodes it with protojson, so the field names are camelCase.
type WebhookEvent struct {
	ID          string              `json:"id"`
	Event       string              `json:"event"`
	CreatedAt   int64               `json:"createdAt"`
	Room        *WebhookRoom        `json:"room"`
	Participant *WebhookParticipant `json:"participant"`
	EgressInfo  json.RawMessage     `json:"egressInfo"`
}

type WebhookRoom struct {
	Name     string `json:"name"`
	Sid      string `json:"sid"`
	Metadata string `json:"metadata"`
}

type WebhookParticipant struct {
	Identity   string            `json:"identity"`
	Kind       string            `json:"kind"`
	Attributes map[string]string `json:"attributes"`
}

// IsSIP reports whether this participant joined over a SIP trunk. Participant
// kind is the strongest signal; the sip.* attributes cover older payloads.
func (p *WebhookParticipant) IsSIP() bool {
	if p == nil {
		return false
	}
	if strings.EqualFold(p.Kind, "SIP") || strings.EqualFold(p.Kind, "PARTICIPANT_KIND_SIP") {
		return true
	}
	for key := range p.Attributes {
		if strings.HasPrefix(key, "sip.") {
			return true
		}
	}
	return false
}

type webhookClaims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Sha256    string `json:"sha256"`
	ExpiresAt int64  `json:"exp"`
	NotBefore int64  `json:"nbf"`
}

// VerifyWebhook validates the Authorization JWT and the body checksum exactly
// as the LiveKit server SDKs do: HS256 with the API secret, then a constant
// time comparison of the sha256 claim against the raw request body.
func VerifyWebhook(apiKey, apiSecret string, body []byte, authHeader string) (WebhookEvent, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
		return WebhookEvent{}, ErrWebhookNotConfigured
	}
	token := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(authHeader), "Bearer "))
	if token == "" {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return WebhookEvent{}, ErrInvalidWebhook
	}

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	wantSignature := mac.Sum(nil)
	gotSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(wantSignature, gotSignature) {
		return WebhookEvent{}, ErrInvalidWebhook
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	var claims webhookClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	if claims.Issuer != apiKey {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	now := time.Now().Unix()
	if claims.ExpiresAt > 0 && claims.ExpiresAt <= now {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	if claims.NotBefore > 0 && claims.NotBefore > now+60 {
		return WebhookEvent{}, ErrInvalidWebhook
	}

	sum := sha256.Sum256(body)
	wantChecksum := base64.StdEncoding.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(claims.Sha256), []byte(wantChecksum)) != 1 {
		return WebhookEvent{}, ErrInvalidWebhook
	}

	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	if event.Event == "" {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	return event, nil
}
