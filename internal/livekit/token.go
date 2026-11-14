package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid livekit token")

type Grants struct {
	Room           string `json:"room,omitempty"`
	RoomJoin       bool   `json:"roomJoin,omitempty"`
	CanPublish     bool   `json:"canPublish,omitempty"`
	CanSubscribe   bool   `json:"canSubscribe,omitempty"`
	CanPublishData bool   `json:"canPublishData,omitempty"`
}

// RoomAgentDispatch asks LiveKit to dispatch a named agent when the room is
// created by the first participant.
type RoomAgentDispatch struct {
	AgentName string `json:"agentName"`
	Metadata  string `json:"metadata,omitempty"`
}

type RoomConfiguration struct {
	Agents []RoomAgentDispatch `json:"agents"`
}

type Options struct {
	APIKey     string
	APISecret  string
	Identity   string
	Name       string
	Metadata   string
	TTL        time.Duration
	Grants     Grants
	RoomConfig *RoomConfiguration
}

type claims struct {
	Issuer     string             `json:"iss"`
	Subject    string             `json:"sub"`
	Name       string             `json:"name,omitempty"`
	Metadata   string             `json:"metadata,omitempty"`
	NotBefore  int64              `json:"nbf"`
	ExpiresAt  int64              `json:"exp"`
	Video      Grants             `json:"video"`
	RoomConfig *RoomConfiguration `json:"roomConfig,omitempty"`
}

// Mint creates a LiveKit room token (JWT HS256) without pulling the full
// server SDK into the API binary.
func Mint(opts Options) (string, error) {
	if strings.TrimSpace(opts.APIKey) == "" || strings.TrimSpace(opts.APISecret) == "" {
		return "", errors.New("livekit api key and secret are required")
	}
	if strings.TrimSpace(opts.Identity) == "" {
		return "", errors.New("livekit identity is required")
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	now := time.Now()
	c := claims{
		Issuer:     opts.APIKey,
		Subject:    opts.Identity,
		Name:       opts.Name,
		Metadata:   opts.Metadata,
		NotBefore:  now.Add(-10 * time.Second).Unix(),
		ExpiresAt:  now.Add(ttl).Unix(),
		Video:      opts.Grants,
		RoomConfig: opts.RoomConfig,
	}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, []byte(opts.APISecret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + signature, nil
}

// Verify checks signature and expiry. Used by tests and by any endpoint that
// needs to trust a token minted elsewhere.
func Verify(apiSecret, token string) (claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(want, got) {
		return claims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims{}, ErrInvalidToken
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return claims{}, ErrInvalidToken
	}
	if c.ExpiresAt <= time.Now().Unix() {
		return claims{}, ErrInvalidToken
	}
	return c, nil
}
