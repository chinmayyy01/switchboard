package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var dummyHash = func() string {
	hash, err := HashPassword("switchboard-invalid-password")
	if err != nil {
		return ""
	}
	return hash
}()

func LoginHandler(store *Store, attempts *AttemptLimiter, ttl time.Duration, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		email := normalizeEmail(body.Email)
		key := clientIP(r) + "|" + email
		if !attempts.Allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many attempts, try again later"})
			return
		}
		account, hash, err := store.FindByEmail(r.Context(), email)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				VerifyPassword(dummyHash, body.Password)
			}
			attempts.Fail(key)
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid email or password"})
			return
		}
		if body.Password == "" || !VerifyPassword(hash, body.Password) {
			attempts.Fail(key)
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid email or password"})
			return
		}
		attempts.Reset(key)
		token, err := store.CreateSession(r.Context(), account.ID, ttl)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not start session"})
			return
		}
		SetSessionCookie(w, token, ttl, secure)
		writeJSON(w, http.StatusOK, map[string]any{"account": account})
	}
}

func LogoutHandler(store *Store, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token := SessionToken(r); token != "" {
			if err := store.DeleteSession(r.Context(), token); err != nil {
				slog.Warn("logout delete session", "err", err)
			}
		}
		ClearSessionCookie(w, secure)
		w.WriteHeader(http.StatusNoContent)
	}
}

func MeHandler(limiter *CallLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"account":     account,
			"active_call": limiter.Active(account.ID),
		})
	}
}

func SessionTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SESSION_TTL"))
	if raw == "" {
		return 12 * time.Hour
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 12 * time.Hour
	}
	return d
}

func envBool(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes")
}
