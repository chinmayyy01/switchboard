package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestLoginAndMe(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "false")
	store, err := Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.CreateAccount(context.Background(), "owner@example.com", "secret", "rivera", false); err != nil {
		t.Fatal(err)
	}
	attempts := NewAttemptLimiter(5, time.Minute)
	limiter := NewCallLimiter()

	body := bytes.NewBufferString(`{"email":"owner@example.com","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	LoginHandler(store, attempts, time.Hour, false)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a session cookie")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(cookies[0])
	meRec := httptest.NewRecorder()
	RequireAccount(store, MeHandler(limiter))(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me status %d: %s", meRec.Code, meRec.Body.String())
	}
	var payload struct {
		Account Account `json:"account"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Account.Email != "owner@example.com" {
		t.Fatalf("unexpected account: %+v", payload.Account)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.CreateAccount(context.Background(), "owner@example.com", "secret", "rivera", false); err != nil {
		t.Fatal(err)
	}
	attempts := NewAttemptLimiter(5, time.Minute)
	body := bytes.NewBufferString(`{"email":"owner@example.com","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	rec := httptest.NewRecorder()
	LoginHandler(store, attempts, time.Hour, false)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("wrong password should not set a cookie")
	}
}

func TestMeRequiresSession(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "false")
	store, err := Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	RequireAccount(store, MeHandler(NewCallLimiter()))(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
