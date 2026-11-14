package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/jobs"
)

func TestStaffManagementIsOwnerOnlyAndStoresWorkRoles(t *testing.T) {
	registry := jobs.NewRegistry("")
	owner := auth.Account{ID: "owner", TenantID: "rivera", IsOwner: true}

	created := callStaff(t, registry, owner, http.MethodPost, "/api/staff?tenant=rivera", `{
"name":"Maya Patel","role":"technician","skills":["leak repair"],
"days":["mon","tue"],"shifts":["morning"],"active":true
}`)
	if created.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var result struct {
		Employee struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"employee"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Employee.ID == "" || result.Employee.Role != "technician" {
		t.Fatalf("employee response=%+v", result.Employee)
	}

	forbidden := callStaff(t, registry, auth.Account{ID: "member", TenantID: "rivera"}, http.MethodPost, "/api/staff", `{"name":"Not allowed","role":"manager"}`)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-owner create status=%d want 403", forbidden.Code)
	}

	deactivated := callStaff(t, registry, owner, http.MethodPatch, "/api/staff/"+result.Employee.ID+"?tenant=rivera", `{"active":false}`)
	if deactivated.Code != http.StatusOK {
		t.Fatalf("deactivate status=%d body=%s", deactivated.Code, deactivated.Body.String())
	}

	listed := callStaff(t, registry, owner, http.MethodGet, "/api/staff?tenant=rivera", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listed.Code, listed.Body.String())
	}
	var members []struct {
		ID     string `json:"id"`
		Active bool   `json:"active"`
		Role   string `json:"role"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &members); err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].ID != result.Employee.ID || members[0].Active || members[0].Role != "technician" {
		t.Fatalf("listed employees=%+v", members)
	}
}

func callStaff(t *testing.T, registry *jobs.Registry, account auth.Account, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request = request.WithContext(auth.WithAccount(context.Background(), account))
	if method == http.MethodPatch || method == http.MethodPut {
		// httptest does not parse ServeMux path values unless it dispatches through one.
		parts := bytes.Split([]byte(request.URL.Path), []byte("/"))
		if len(parts) >= 4 {
			request.SetPathValue("id", string(parts[3]))
		}
	}
	recorder := httptest.NewRecorder()
	Staff(registry)(recorder, request)
	return recorder
}
