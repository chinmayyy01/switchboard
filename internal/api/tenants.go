package api

import (
	"net/http"

	"github.com/chinmay/switchboard/internal/tenant"
)

// Tenants lists the workspaces the signed-in account may use.
func Tenants(reg *tenant.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		out := []map[string]string{}
		if account.IsOwner {
			for _, item := range reg.All() {
				out = append(out, map[string]string{"id": item.ID, "name": item.Name})
			}
		} else if item, ok := reg.Get(account.TenantID); ok {
			out = append(out, map[string]string{"id": item.ID, "name": item.Name})
		}
		writeJSON(w, out)
	}
}
