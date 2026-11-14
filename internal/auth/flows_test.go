package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"

	tenantpkg "github.com/chinmay/switchboard/internal/tenant"
	"testing"
	"time"
)

type fakeSender struct {
	to      string
	subject string
	body    string
	count   int
}

func (f *fakeSender) Send(_ context.Context, to, subject, body string) error {
	f.to, f.subject, f.body = to, subject, body
	f.count++
	return nil
}

func tokenFromBody(t *testing.T, body string) string {
	t.Helper()
	for _, field := range bytes.Fields([]byte(body)) {
		value := string(field)
		if !bytes.Contains(field, []byte("token=")) {
			continue
		}
		u, err := url.Parse(value)
		if err != nil {
			continue
		}
		if token := u.Query().Get("token"); token != "" {
			return token
		}
	}
	t.Fatalf("no token in email body: %q", body)
	return ""
}

func TestInviteSignupVerifyAndLogin(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	t.Setenv("APP_ENV", "development")
	ctx := context.Background()
	store, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenant := "flow-test-" + time.Now().Format("150405.000000")
	email := tenant + "@example.com"
	t.Cleanup(func() {
		store.db.ExecContext(context.Background(), "DELETE FROM auth_tokens WHERE account_id IN (SELECT id FROM accounts WHERE tenant_id = $1)", tenant)
		store.db.ExecContext(context.Background(), "DELETE FROM invites WHERE tenant_id = $1", tenant)
		store.db.ExecContext(context.Background(), "DELETE FROM sessions WHERE account_id IN (SELECT id FROM accounts WHERE tenant_id = $1)", tenant)
		store.db.ExecContext(context.Background(), "DELETE FROM accounts WHERE tenant_id = $1", tenant)
		store.Close()
	})

	inviteToken, err := store.CreateInvite(ctx, email, tenant, RoleViewer, "owner", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	invite, err := store.InviteByToken(ctx, inviteToken)
	if err != nil || invite.Email != email || invite.Role != RoleViewer {
		t.Fatalf("invite = %+v err=%v", invite, err)
	}

	sender := &fakeSender{}
	body, _ := json.Marshal(map[string]string{
		"token": inviteToken, "name": "Test Person", "password": "correct horse battery",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	SignupHandler(store, sender, NewAttemptLimiter(20, time.Minute), tenantpkg.New(), t.TempDir())(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup status=%d body=%s", rec.Code, rec.Body.String())
	}
	var signup struct {
		OK      bool   `json:"ok"`
		DevLink string `json:"dev_link"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &signup); err != nil {
		t.Fatal(err)
	}
	if !signup.OK || signup.DevLink == "" {
		t.Fatalf("signup = %+v", signup)
	}
	account, _, err := store.FindByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	if account.EmailVerified || account.Role != RoleViewer || account.Name != "Test Person" {
		t.Fatalf("account = %+v", account)
	}

	// Unverified accounts cannot sign in.
	loginBody, _ := json.Marshal(map[string]string{"email": email, "password": "correct horse battery"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	LoginHandler(store, NewAttemptLimiter(20, time.Minute), time.Hour, false)(loginRec, loginReq)
	if loginRec.Code != http.StatusForbidden {
		t.Fatalf("unverified login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}

	verifyToken := tokenFromBody(t, sender.body)
	verifyBody, _ := json.Marshal(map[string]string{"token": verifyToken})
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	VerifyHandler(store, time.Hour, false)(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify status=%d body=%s", verifyRec.Code, verifyRec.Body.String())
	}
	account, _, err = store.FindByEmail(ctx, email)
	if err != nil || !account.EmailVerified {
		t.Fatalf("verified account = %+v err=%v", account, err)
	}

	// The verify response sets a session cookie.
	if len(verifyRec.Result().Cookies()) == 0 {
		t.Fatal("verify did not set a session cookie")
	}

	// Password reset.
	forgotBody, _ := json.Marshal(map[string]string{"email": email})
	forgotReq := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(forgotBody))
	forgotRec := httptest.NewRecorder()
	cooldown := NewAttemptLimiter(1, time.Minute)
	ForgotPasswordHandler(store, sender, NewAttemptLimiter(20, time.Minute), cooldown)(forgotRec, forgotReq)
	if forgotRec.Code != http.StatusNoContent && forgotRec.Code != http.StatusOK {
		t.Fatalf("forgot status=%d", forgotRec.Code)
	}
	sentBefore := sender.count
	// A second request inside the cooldown must not send another email.
	secondRec := httptest.NewRecorder()
	ForgotPasswordHandler(store, sender, NewAttemptLimiter(20, time.Minute), cooldown)(secondRec, httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(forgotBody)))
	if sender.count != sentBefore {
		t.Fatalf("cooldown sent %d extra email(s)", sender.count-sentBefore)
	}
	if secondRec.Code != http.StatusNoContent {
		t.Fatalf("cooldown status=%d", secondRec.Code)
	}
	resetToken := tokenFromBody(t, sender.body)
	resetBody, _ := json.Marshal(map[string]string{"token": resetToken, "password": "a new correct horse"})
	resetReq := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(resetBody))
	resetRec := httptest.NewRecorder()
	ResetPasswordHandler(store)(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset status=%d body=%s", resetRec.Code, resetRec.Body.String())
	}
	loginBody, _ = json.Marshal(map[string]string{"email": email, "password": "a new correct horse"})
	loginReq = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginRec = httptest.NewRecorder()
	LoginHandler(store, NewAttemptLimiter(20, time.Minute), time.Hour, false)(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login after reset status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
}

func TestPublicSignupCreatesTenantAndOwner(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	t.Setenv("APP_ENV", "development")
	ctx := context.Background()
	store, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	registry := tenantpkg.New()
	tenantDir := t.TempDir()
	email := "public-" + time.Now().Format("150405.000000") + "@example.com"
	t.Cleanup(func() {
		store.db.ExecContext(context.Background(), "DELETE FROM auth_tokens WHERE account_id IN (SELECT id FROM accounts WHERE email = $1)", email)
		store.db.ExecContext(context.Background(), "DELETE FROM sessions WHERE account_id IN (SELECT id FROM accounts WHERE email = $1)", email)
		store.db.ExecContext(context.Background(), "DELETE FROM accounts WHERE email = $1", email)
		store.Close()
	})

	body, _ := json.Marshal(map[string]string{
		"name": "Public Owner", "business": "Northstar Plumbing",
		"email": email, "password": "correct horse battery",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	SignupHandler(store, &fakeSender{}, NewAttemptLimiter(20, time.Minute), registry, tenantDir)(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		TenantID string `json:"tenant_id"`
		DevLink  string `json:"dev_link"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.TenantID == "" || response.DevLink == "" {
		t.Fatalf("response = %+v", response)
	}
	cfg, ok := registry.Get(response.TenantID)
	if !ok || cfg.Name != "Northstar Plumbing" {
		t.Fatalf("tenant not registered: %+v ok=%v", cfg, ok)
	}
	if _, err := os.Stat(tenantDir + "/" + response.TenantID + "/config.json"); err != nil {
		t.Fatalf("tenant config was not written: %v", err)
	}
	account, _, err := store.FindByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	if account.Role != RoleOwner || account.IsOwner || account.EmailVerified || account.TenantID != response.TenantID {
		t.Fatalf("account = %+v", account)
	}
}
