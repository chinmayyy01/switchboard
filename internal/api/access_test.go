package api

import (
	"testing"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/hub"
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

func TestCanSeeCall(t *testing.T) {
	account := auth.Account{ID: "a", TenantID: "rivera"}
	if !canSeeCall(account, hub.Call{Tenant: "rivera", AccountID: "a"}) {
		t.Fatal("account should see own call")
	}
	if canSeeCall(account, hub.Call{Tenant: "rivera", AccountID: "b"}) {
		t.Fatal("account should not see another call")
	}
}
