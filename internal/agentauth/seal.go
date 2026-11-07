package agentauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidSeal = errors.New("invalid sealed payload")

type sealed struct {
	Payload   json.RawMessage `json:"payload"`
	ExpiresAt int64           `json:"exp"`
}

// Seal signs an arbitrary payload for short-lived, stateless hand-offs. Used
// for booking confirmations so the agent can only book details that were
// validated by the server and read back to the caller.
func Seal(secret string, value any, ttl time.Duration) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("seal secret is required")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(sealed{
		Payload:   payload,
		ExpiresAt: time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func Open(secret, token string, value any) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return ErrInvalidSeal
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(want, got) {
		return ErrInvalidSeal
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ErrInvalidSeal
	}
	var wrapper sealed
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return ErrInvalidSeal
	}
	if wrapper.ExpiresAt <= time.Now().Unix() {
		return ErrInvalidSeal
	}
	return json.Unmarshal(wrapper.Payload, value)
}
