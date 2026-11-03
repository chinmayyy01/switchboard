package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const CookieName = "sb_session"

type accountContextKey struct{}

func WithAccount(ctx context.Context, account Account) context.Context {
	return context.WithValue(ctx, accountContextKey{}, account)
}

func FromContext(ctx context.Context) (Account, bool) {
	account, ok := ctx.Value(accountContextKey{}).(Account)
	return account, ok
}

func Disabled() bool {
	return envBool("AUTH_DISABLED", false)
}

func DevAccount() Account {
	tenant := "rivera"
	if v := strings.TrimSpace(os.Getenv("DEV_TENANT")); v != "" {
		tenant = v
	}
	return Account{
		ID:            "dev",
		Email:         "dev@switchboard.local",
		TenantID:      tenant,
		IsOwner:       true,
		FreeCallLimit: 0,
	}
}

func RequireAccount(store *Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if Disabled() {
			next(w, r.WithContext(WithAccount(r.Context(), DevAccount())))
			return
		}
		token := SessionToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required"})
			return
		}
		account, err := store.AccountForToken(r.Context(), token)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				ClearSessionCookie(w, SecureCookie())
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "session expired"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "auth unavailable"})
			return
		}
		next(w, r.WithContext(WithAccount(r.Context(), account)))
	}
}

func SessionToken(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func SetSessionCookie(w http.ResponseWriter, token string, ttl time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func SecureCookie() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		return true
	}
	return envBool("COOKIE_SECURE", false)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if i := len(forwarded); i > 0 {
			for j := 0; j < len(forwarded); j++ {
				if forwarded[j] == ',' {
					return forwarded[:j]
				}
			}
			return forwarded
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
