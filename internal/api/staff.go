package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/staff"
)

type employeePayload struct {
	Name   string   `json:"name"`
	Role   string   `json:"role"`
	Email  string   `json:"email"`
	Phone  string   `json:"phone"`
	Skills []string `json:"skills"`
	Days   []string `json:"days"`
	Shifts []string `json:"shifts"`
	Active *bool    `json:"active"`
}

func Staff(reg *jobs.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tenant := account.TenantID
		if account.IsOwner {
			tenant = strings.TrimSpace(r.URL.Query().Get("tenant"))
			if tenant == "" {
				http.Error(w, "tenant required", http.StatusBadRequest)
				return
			}
		}
		store := reg.For(tenant)

		switch r.Method {
		case http.MethodGet:
			employees, err := store.EmployeesContext(r.Context())
			if err != nil {
				http.Error(w, "could not load team", http.StatusInternalServerError)
				return
			}
			writeJSON(w, employees)

		case http.MethodPost, http.MethodPut:
			if !account.IsOwner {
				http.Error(w, "only the workspace owner can manage employees", http.StatusForbidden)
				return
			}
			var payload employeePayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid employee details", http.StatusBadRequest)
				return
			}
			active := true
			if payload.Active != nil {
				active = *payload.Active
			}
			employee, err := staff.Normalize(staff.Employee{
				ID:     r.PathValue("id"),
				Name:   payload.Name,
				Role:   payload.Role,
				Email:  payload.Email,
				Phone:  payload.Phone,
				Skills: payload.Skills,
				Days:   payload.Days,
				Shifts: payload.Shifts,
				Active: active,
			})
			if err != nil {
				http.Error(w, "name, work role, and valid schedule details are required", http.StatusBadRequest)
				return
			}
			saved, err := store.SaveEmployeeContext(r.Context(), employee)
			if err != nil {
				if errors.Is(err, staff.ErrInvalidEmployee) {
					http.Error(w, "name, work role, and valid schedule details are required", http.StatusBadRequest)
					return
				}
				http.Error(w, "could not save employee", http.StatusConflict)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "employee": saved})

		case http.MethodPatch:
			if !account.IsOwner {
				http.Error(w, "only the workspace owner can manage employees", http.StatusForbidden)
				return
			}
			var payload struct {
				Active bool `json:"active"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid employee status", http.StatusBadRequest)
				return
			}
			if err := store.SetEmployeeActiveContext(r.Context(), r.PathValue("id"), payload.Active); err != nil {
				if errors.Is(err, jobs.ErrNotFound) {
					http.Error(w, "employee not found", http.StatusNotFound)
					return
				}
				http.Error(w, "could not update employee", http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]any{"ok": true})

		default:
			w.Header().Set("Allow", "GET, POST, PUT, PATCH, OPTIONS")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
