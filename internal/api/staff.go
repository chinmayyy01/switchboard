package api

import (
	"net/http"

	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/staff"
)

type StaffMember struct {
	staff.Employee
	Busy []string `json:"busy"`
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
			tenant = r.URL.Query().Get("tenant")
			if tenant == "" {
				http.Error(w, "tenant required", http.StatusBadRequest)
				return
			}
		}
		store := reg.For(tenant)
		if store.Staff == nil {
			writeJSON(w, []StaffMember{})
			return
		}
		busy := store.Staff.Busy()
		emps := store.Staff.Employees()
		out := make([]StaffMember, 0, len(emps))
		for _, e := range emps {
			b := busy[e.ID]
			if b == nil {
				b = []string{}
			}
			out = append(out, StaffMember{Employee: e, Busy: b})
		}
		writeJSON(w, out)
	}
}
