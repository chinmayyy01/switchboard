package api

import (
	"net/http"

	"github.com/chinmay/switchboard/internal/auth"
)

func canSee(account auth.Account, tenant, accountID string) bool {
	if account.IsOwner {
		return true
	}
	return accountID == account.ID
}

func canSeeTenant(account auth.Account, tenant string) bool {
	return account.IsOwner || account.TenantID == tenant
}

func accountFrom(r *http.Request) (auth.Account, bool) {
	return auth.FromContext(r.Context())
}
