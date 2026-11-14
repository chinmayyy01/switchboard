package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/tenant"
)

type TenantAPI struct {
	Tenants *tenant.Registry
	Dir     string
}

func (a *TenantAPI) tenantID(r *http.Request, account auth.Account) string {
	if account.IsOwner {
		if value := strings.TrimSpace(r.URL.Query().Get("tenant")); value != "" {
			return value
		}
	}
	return account.TenantID
}

func (a *TenantAPI) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		cfg, ok := a.Tenants.Get(a.tenantID(r, account))
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		writeJSON(w, cfg)
	}
}

// Update edits the safe, tenant-facing configuration. The tenant ID, telephony
// settings, and manager routing are not changed here.
func (a *TenantAPI) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !account.IsOwner && account.Role != auth.RoleOwner {
			http.Error(w, "owner role required", http.StatusForbidden)
			return
		}
		dir := a.Dir
		if dir == "" {
			dir = "tenants"
		}
		id := a.tenantID(r, account)
		existing, ok := a.Tenants.Get(id)
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		var body struct {
			Name          *string         `json:"name"`
			Services      *[]string       `json:"services"`
			Windows       *[]string       `json:"windows"`
			Persona       *tenant.Persona `json:"persona"`
			Disclosure    *string         `json:"disclosure"`
			ManagerNumber *string         `json:"manager_number"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid settings", http.StatusBadRequest)
			return
		}
		if body.Name != nil {
			existing.Name = *body.Name
		}
		if body.Services != nil {
			existing.Services = *body.Services
		}
		if body.Windows != nil {
			existing.Windows = *body.Windows
		}
		if body.Persona != nil {
			existing.Persona = *body.Persona
		}
		if body.Disclosure != nil {
			existing.Disclosure = *body.Disclosure
		}
		if body.ManagerNumber != nil {
			existing.ManagerNumber = *body.ManagerNumber
		}
		saved, err := tenant.Save(dir, existing)
		if err != nil {
			switch {
			case errors.Is(err, tenant.ErrInvalidName), errors.Is(err, tenant.ErrInvalidServices), errors.Is(err, tenant.ErrInvalidWindows):
				http.Error(w, err.Error(), http.StatusBadRequest)
			default:
				http.Error(w, "could not save settings", http.StatusInternalServerError)
			}
			return
		}
		a.Tenants.Add(saved)
		writeJSON(w, saved)
	}
}
