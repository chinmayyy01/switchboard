package auth

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPostgresAuthStore(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := "auth-test-" + time.Now().Format("150405.000000")
	email := tenantID + "@example.com"
	t.Cleanup(func() {
		store.db.ExecContext(context.Background(), "DELETE FROM sessions WHERE account_id IN (SELECT id FROM accounts WHERE tenant_id = $1)", tenantID)
		store.db.ExecContext(context.Background(), "DELETE FROM accounts WHERE tenant_id = $1", tenantID)
		store.Close()
	})

	account, err := store.CreateAccountWithRole(ctx, "Test Viewer", email, "correct horse battery", tenantID, RoleViewer, true)
	if err != nil {
		t.Fatal(err)
	}
	if account.IsOwner || account.FreeCallLimit != 2 {
		t.Fatalf("account = %+v", account)
	}
	found, hash, err := store.FindByEmail(ctx, email)
	if err != nil || found.ID != account.ID || hash == "" {
		t.Fatalf("find = %+v hash=%q err=%v", found, hash, err)
	}
	if !VerifyPassword(hash, "correct horse battery") {
		t.Fatal("password hash did not verify")
	}
	allowed, err := store.StartCall(ctx, account.ID)
	if err != nil || !allowed {
		t.Fatalf("start call allowed=%v err=%v", allowed, err)
	}
	token, err := store.CreateSession(ctx, account.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	byToken, err := store.AccountForToken(ctx, token)
	if err != nil || byToken.ID != account.ID {
		t.Fatalf("account for token = %+v err=%v", byToken, err)
	}
	if err := store.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AccountForToken(ctx, token); err != ErrNotFound {
		t.Fatalf("deleted session still valid: %v", err)
	}

	exported, err := store.ExportAccounts(ctx)
	if err != nil || len(exported) == 0 {
		t.Fatalf("export = %d err=%v", len(exported), err)
	}
}
