package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestAccountsAndSessions(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	account, err := store.CreateAccount(ctx, "Owner@Example.com", "secret", "rivera", false)
	if err != nil {
		t.Fatal(err)
	}
	if account.Email != "owner@example.com" {
		t.Fatalf("email not normalized: %q", account.Email)
	}
	if account.IsOwner {
		t.Fatal("account should not be owner")
	}
	if account.FreeCallLimit != 2 {
		t.Fatalf("expected 2 free calls, got %d", account.FreeCallLimit)
	}

	found, hash, err := store.FindByEmail(ctx, "owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != account.ID {
		t.Fatalf("wrong account: %s", found.ID)
	}
	if !VerifyPassword(hash, "secret") {
		t.Fatal("stored password should verify")
	}

	token, err := store.CreateSession(ctx, account.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.AccountForToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != account.ID {
		t.Fatalf("wrong session account: %s", session.ID)
	}
	if _, err := store.AccountForToken(ctx, "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}

	if err := store.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AccountForToken(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected deleted session, got %v", err)
	}
}

func TestExpiredSession(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	account, err := store.CreateAccount(ctx, "a@b.test", "secret", "rivera", false)
	if err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, account.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE sessions SET expires_at = ?", time.Now().Add(-time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AccountForToken(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected expired session, got %v", err)
	}
}

func TestStartCallQuota(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	account, err := store.CreateAccount(ctx, "trial@test", "secret", "rivera", false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		ok, err := store.StartCall(ctx, account.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("call %d should be allowed", i+1)
		}
	}
	ok, err := store.StartCall(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("third call should be blocked")
	}
	found, _, err := store.FindByEmail(ctx, "trial@test")
	if err != nil {
		t.Fatal(err)
	}
	if found.CallsUsed != 2 {
		t.Fatalf("expected 2 calls used, got %d", found.CallsUsed)
	}
}

func TestOwnerStartsUnlimitedCalls(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner, err := store.CreateAccount(ctx, "owner@test", "secret", "rivera", true)
	if err != nil {
		t.Fatal(err)
	}
	if owner.FreeCallLimit != 0 {
		t.Fatalf("owner should have no limit, got %d", owner.FreeCallLimit)
	}
	for i := 0; i < 5; i++ {
		ok, err := store.StartCall(ctx, owner.ID)
		if err != nil || !ok {
			t.Fatalf("owner call %d failed: %v", i+1, err)
		}
	}
}
