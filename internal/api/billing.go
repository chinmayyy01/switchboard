package api

import (
	"encoding/json"
	"net/http"

	"github.com/chinmay/switchboard/internal/billing"
	"github.com/chinmay/switchboard/internal/tenant"
)

type BillingAPI struct {
	Billing *billing.Service
	Tenants *tenant.Registry
}

func (a *BillingAPI) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !canEditTenant(account) {
			http.Error(w, "owner role required", http.StatusForbidden)
			return
		}
		id := requestTenantID(r, account)
		if _, ok := a.Tenants.Get(id); !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		if a.Billing == nil {
			http.Error(w, "billing is not configured", http.StatusServiceUnavailable)
			return
		}
		summary, err := a.Billing.Summary(r.Context(), id)
		if err != nil {
			http.Error(w, "could not load usage", http.StatusInternalServerError)
			return
		}
		writeJSON(w, summary)
	}
}

// Update sets the monthly cap and the action taken when it is reached.
// cap_minor is in the smallest currency unit; zero means unlimited.
func (a *BillingAPI) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !canEditTenant(account) {
			http.Error(w, "owner role required", http.StatusForbidden)
			return
		}
		id := requestTenantID(r, account)
		if _, ok := a.Tenants.Get(id); !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		if a.Billing == nil {
			http.Error(w, "billing is not configured", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			CapMinor *int64 `json:"cap_minor"`
			Action   string `json:"action"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CapMinor == nil {
			http.Error(w, "invalid budget", http.StatusBadRequest)
			return
		}
		if err := a.Billing.SetBudget(r.Context(), id, *body.CapMinor, body.Action); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		summary, err := a.Billing.Summary(r.Context(), id)
		if err != nil {
			http.Error(w, "could not load usage", http.StatusInternalServerError)
			return
		}
		writeJSON(w, summary)
	}
}
