package api

import (
	"testing"

	"github.com/chinmay/switchboard/internal/auth"
)

func TestCanSee(t *testing.T) {
	owner := auth.Account{ID: "a", TenantID: "rivera"}
	if !canSee(owner, "rivera", "a") {
		t.Fatal("account should see own call")
	}
	if canSee(owner, "rivera", "other") {
		t.Fatal("account should not see another account's call")
	}

	admin := auth.Account{ID: "d", IsOwner: true}
	if !canSee(admin, "acme", "a") {
		t.Fatal("owner should see any call")
	}
}

func TestCanSeeTenant(t *testing.T) {
	account := auth.Account{ID: "a", TenantID: "rivera"}
	if !canSeeTenant(account, "rivera") {
		t.Fatal("account should see its tenant")
	}
	if canSeeTenant(account, "acme") {
		t.Fatal("account should not see another tenant")
	}
	admin := auth.Account{ID: "d", IsOwner: true}
	if !canSeeTenant(admin, "acme") {
		t.Fatal("owner should see any tenant")
	}
}
