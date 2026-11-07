package agentauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid agent token")

type Claims struct {
	TenantID  string `json:"tenant_id"`
	CallID    string `json:"call_id"`
	Direction string `json:"direction,omitempty"`
	ExpiresAt int64  `json:"exp"`
}

type contextKey struct{}

func WithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, claims)
}

func FromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(contextKey{}).(Claims)
	return claims, ok
}

// Mint signs a short-lived per-call token. The agent exchanges its bootstrap
// key for one of these at the start of each job, so tool credentials are
// scoped to a single tenant and call.
func Mint(secret string, claims Claims) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("agent token secret is required")
	}
	if claims.TenantID == "" || claims.CallID == "" {
		return "", errors.New("tenant and call are required")
	}
	if claims.ExpiresAt == 0 {
		claims.ExpiresAt = time.Now().Add(time.Hour).Unix()
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func Verify(secret, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(want, got) {
		return Claims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.TenantID == "" || claims.CallID == "" || claims.ExpiresAt <= time.Now().Unix() {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

// BootstrapKey compares the agent's provisioning key in constant time.
func BootstrapKey(got, want string) bool {
	if want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func Require(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
		if token == "" || token == header {
			http.Error(w, "missing agent token", http.StatusUnauthorized)
			return
		}
		claims, err := Verify(secret, token)
		if err != nil {
			http.Error(w, "invalid agent token", http.StatusUnauthorized)
			return
		}
		next(w, r.WithContext(WithClaims(r.Context(), claims)))
	}
}
