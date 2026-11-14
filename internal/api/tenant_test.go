package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/tenant"
)

func TestTenantSettingsUpdate(t *testing.T) {
	dir := t.TempDir()
	cfg := tenant.Tenant{
		ID:       "settings-test",
		Name:     "Old Name",
		Services: []string{"repair"},
		Windows:  []string{"morning"},
	}
	if _, err := tenant.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	registry := tenant.New(cfg)
	api := &TenantAPI{Tenants: registry, Dir: dir}
	account := auth.Account{ID: "owner-1", TenantID: cfg.ID, Role: auth.RoleOwner}

	body, _ := json.Marshal(map[string]any{
		"name":     "New Name",
		"services": []string{"repair", "installation"},
		"windows":  []string{"morning", "evening"},
		"persona":  map[string]any{"agent_name": "Maya", "tone": "calm", "languages": []string{"en", "hi"}},
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/tenant?tenant=settings-test", bytes.NewReader(body))
	req = req.WithContext(auth.WithAccount(req.Context(), account))
	rec := httptest.NewRecorder()
	api.Update()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", rec.Code, rec.Body.String())
	}
	updated, ok := registry.Get(cfg.ID)
	if !ok || updated.Name != "New Name" || len(updated.Services) != 2 || updated.Persona.AgentName != "Maya" {
		t.Fatalf("updated = %+v ok=%v", updated, ok)
	}
	reloaded, err := tenant.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := reloaded.Get(cfg.ID); !ok || got.Name != "New Name" {
		t.Fatalf("reloaded = %+v ok=%v", got, ok)
	}

	// A viewer cannot edit.
	viewer := auth.Account{ID: "viewer", TenantID: cfg.ID, Role: auth.RoleViewer}
	req = httptest.NewRequest(http.MethodPatch, "/api/tenant", bytes.NewReader(body))
	req = req.WithContext(auth.WithAccount(req.Context(), viewer))
	rec = httptest.NewRecorder()
	api.Update()(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer status=%d", rec.Code)
	}
}
