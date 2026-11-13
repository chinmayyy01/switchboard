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
	if !envBool("AUTH_DISABLED", false) {
		return false
	}
	// Never let a stray AUTH_DISABLED=true bypass auth in production.
	if envBool("ALLOW_DEV_AUTH", false) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "development")
}

func DevAccount() Account {
	tenant := "rivera"
	if v := strings.TrimSpace(os.Getenv("DEV_TENANT")); v != "" {
		tenant = v
	}
	return Account{
		ID:            "dev",
		Email:         "dev@switchboard.local",
		Name:          "Local preview",
		TenantID:      tenant,
		Role:          RoleOwner,
		IsOwner:       true,
		FreeCallLimit: 0,
		EmailVerified: true,
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

func RequireRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	allowed := map[string]bool{}
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if Disabled() {
				next(w, r.WithContext(WithAccount(r.Context(), DevAccount())))
				return
			}
			account, ok := FromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required"})
				return
			}
			if !allowed[account.Role] {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "insufficient role"})
				return
			}
			next(w, r)
		}
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !trustedProxy(host) {
		return host
	}
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return host
	}
	for i := 0; i < len(forwarded); i++ {
		if forwarded[i] == ',' {
			return strings.TrimSpace(forwarded[:i])
		}
	}
	return strings.TrimSpace(forwarded)
}

// trustedProxy only honours X-Forwarded-For when the immediate peer is in
// TRUSTED_PROXIES. Without that list the header is ignored, so a client cannot
// spoof its address to reset the login limiter.
func trustedProxy(host string) bool {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if raw == "" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if candidate := net.ParseIP(entry); candidate != nil {
			if candidate.Equal(ip) {
				return true
			}
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}
