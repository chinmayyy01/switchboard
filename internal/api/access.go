package api

import (
	"net/http"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/hub"
)

func canSee(account auth.Account, tenant, accountID string) bool {
	if account.IsOwner {
		return true
	}
	return accountID == account.ID
}

func accountFrom(r *http.Request) (auth.Account, bool) {
	return auth.FromContext(r.Context())
}

func canSeeCall(account auth.Account, call hub.Call) bool {
	return canSee(account, call.Tenant, call.AccountID)
}
