package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/mailer"
	"github.com/chinmay/switchboard/internal/tenant"
)

func devLink(path, token string) string {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "development") {
		return ""
	}
	return mailer.Link(path, token)
}

// InviteInfo is public: it only reveals the invited email, tenant, and role.
func InviteInfoHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invite, err := store.InviteByToken(r.Context(), r.URL.Query().Get("token"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "invite is invalid or expired"})
			return
		}
		writeJSON(w, http.StatusOK, invite)
	}
}

func SignupHandler(store *Store, sender mailer.Sender, attempts *AttemptLimiter, tenants *tenant.Registry, tenantDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "signup|" + clientIP(r)
		if !attempts.Allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many attempts, try again later"})
			return
		}
		attempts.Fail(key)
		var body struct {
			Token    string `json:"token"`
			Name     string `json:"name"`
			Business string `json:"business"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		if len(body.Password) < 8 || len(body.Password) > 200 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "password must be 8–200 characters"})
			return
		}

		var account Account
		response := map[string]any{}
		if strings.TrimSpace(body.Token) != "" {
			// Existing workspace: accept an owner/manager invite.
			invite, err := store.InviteByToken(r.Context(), strings.TrimSpace(body.Token))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invite is invalid or expired"})
				return
			}
			account, err = store.CreateAccountWithRole(
				r.Context(), body.Name, invite.Email, body.Password, invite.TenantID, invite.Role, false,
			)
			if err != nil {
				writeSignupError(w, err)
				return
			}
			if err := store.AcceptInvite(r.Context(), body.Token); err != nil {
				slog.Warn("accept invite", "err", err)
			}
		} else {
			// New workspace: every signup gets its own tenant and owner account.
			if tenants == nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "signup is not configured"})
				return
			}
			if strings.TrimSpace(body.Business) == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "business name is required"})
				return
			}
			if _, _, err := store.FindByEmail(r.Context(), body.Email); err == nil {
				writeJSON(w, http.StatusConflict, map[string]any{"error": "an account with this email already exists"})
				return
			} else if !errors.Is(err, ErrNotFound) {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not create the account"})
				return
			}
			cfg, err := tenant.Provision(tenantDir, body.Business, func(id string) bool {
				_, ok := tenants.Get(id)
				return ok
			})
			if err != nil {
				slog.Error("provision tenant", "err", err)
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not create the workspace"})
				return
			}
			account, err = store.CreateOwnerAccount(r.Context(), body.Name, body.Email, body.Password, cfg.ID, false)
			if err != nil {
				writeSignupError(w, err)
				return
			}
			tenants.Add(cfg)
			response["tenant_id"] = cfg.ID
			response["tenant_name"] = cfg.Name
		}

		verify, err := store.CreateAuthToken(r.Context(), account.ID, "verify", 24*time.Hour)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not create a verification token"})
			return
		}
		link := mailer.Link("/verify", verify)
		if err := sender.Send(r.Context(), account.Email,
			"Verify your Switchboard account",
			"Welcome to Switchboard.\n\nVerify your email:\n"+link+"\n\nThis link expires in 24 hours."); err != nil {
			slog.Error("send verification email", "email", account.Email, "err", err)
		}
		response["ok"] = true
		response["email"] = account.Email
		response["verification_required"] = true
		response["dev_link"] = devLink("/verify", verify)
		writeJSON(w, http.StatusCreated, response)
	}
}

func writeSignupError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrEmailTaken) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "an account with this email already exists"})
		return
	}
	slog.Error("signup", "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not create the account"})
}

func VerifyHandler(store *Store, ttl time.Duration, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		accountID, err := store.ConsumeAuthToken(r.Context(), body.Token, "verify")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "verification link is invalid or expired"})
			return
		}
		if err := store.MarkEmailVerified(r.Context(), accountID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not verify the account"})
			return
		}
		account, err := store.AccountByID(r.Context(), accountID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not load the account"})
			return
		}
		token, err := store.CreateSession(r.Context(), account.ID, ttl)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not start a session"})
			return
		}
		SetSessionCookie(w, token, ttl, secure)
		writeJSON(w, http.StatusOK, map[string]any{"account": account})
	}
}

func ResendVerificationHandler(store *Store, sender mailer.Sender, attempts *AttemptLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "resend|" + clientIP(r)
		if !attempts.Allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many attempts, try again later"})
			return
		}
		attempts.Fail(key)
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		account, _, err := store.FindByEmail(r.Context(), body.Email)
		if err == nil && !account.EmailVerified {
			if token, err := store.CreateAuthToken(r.Context(), account.ID, "verify", 24*time.Hour); err == nil {
				link := mailer.Link("/verify", token)
				_ = sender.Send(r.Context(), account.Email, "Verify your Switchboard account",
					"Verify your email:\n"+link+"\n\nThis link expires in 24 hours.")
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func ForgotPasswordHandler(store *Store, sender mailer.Sender, attempts *AttemptLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "forgot|" + clientIP(r)
		if !attempts.Allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many attempts, try again later"})
			return
		}
		attempts.Fail(key)
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		account, _, err := store.FindByEmail(r.Context(), body.Email)
		if err == nil {
			if token, err := store.CreateAuthToken(r.Context(), account.ID, "reset", time.Hour); err == nil {
				link := mailer.Link("/reset-password", token)
				_ = sender.Send(r.Context(), account.Email, "Reset your Switchboard password",
					"Reset your password:\n"+link+"\n\nThis link expires in one hour. If you did not ask, ignore this email.")
			}
		}
		// Never reveal whether an account exists.
		w.WriteHeader(http.StatusNoContent)
	}
}

func ResetPasswordHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		if len(body.Password) < 8 || len(body.Password) > 200 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "password must be 8–200 characters"})
			return
		}
		accountID, err := store.ConsumeAuthToken(r.Context(), body.Token, "reset")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "reset link is invalid or expired"})
			return
		}
		if err := store.SetPassword(r.Context(), accountID, body.Password); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not update the password"})
			return
		}
		_ = store.DeleteSessionsForAccount(r.Context(), accountID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func InviteHandler(store *Store, sender mailer.Sender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required"})
			return
		}
		if account.Role != RoleOwner {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "owner only"})
			return
		}
		var body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		if body.Role == "" {
			body.Role = RoleViewer
		}
		token, err := store.CreateInvite(r.Context(), body.Email, account.TenantID, body.Role, account.ID, 7*24*time.Hour)
		if err != nil {
			if errors.Is(err, ErrInvalidRole) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "role must be manager or viewer"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not create the invite"})
			return
		}
		inviteURL := mailer.Link("/signup", token)
		if err := sender.Send(r.Context(), body.Email, "You're invited to Switchboard",
			"You have been invited to Switchboard.\n\nAccept the invite:\n"+inviteURL+"\n\nThis link expires in 7 days."); err != nil {
			slog.Error("send invite", "email", body.Email, "err", err)
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"ok":         true,
			"invite_url": inviteURL,
			"expires_in": 7 * 24 * 60 * 60,
		})
	}
}

func TeamAccountsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required"})
			return
		}
		accounts, err := store.ListAccounts(r.Context(), account.TenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not load accounts"})
			return
		}
		writeJSON(w, http.StatusOK, accounts)
	}
}

func UpdateAccountHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required"})
			return
		}
		if actor.Role != RoleOwner {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "owner only"})
			return
		}
		target, err := store.AccountByID(r.Context(), r.PathValue("id"))
		if err != nil || target.TenantID != actor.TenantID {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "account not found"})
			return
		}
		if target.IsOwner {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "the owner role cannot be changed"})
			return
		}
		var body struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		if err := store.UpdateRole(r.Context(), target.ID, body.Role); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid role"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func DeleteAccountHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required"})
			return
		}
		if actor.Role != RoleOwner {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "owner only"})
			return
		}
		target, err := store.AccountByID(r.Context(), r.PathValue("id"))
		if err != nil || target.TenantID != actor.TenantID {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "account not found"})
			return
		}
		if target.IsOwner {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "the owner cannot be removed"})
			return
		}
		if err := store.DeleteAccount(r.Context(), target.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not remove the account"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func TeamInvitesHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in required"})
			return
		}
		invites, err := store.ListInvites(r.Context(), account.TenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not load invites"})
			return
		}
		writeJSON(w, http.StatusOK, invites)
	}
}
