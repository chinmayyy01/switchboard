package tenant

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveValidatesAndPersists(t *testing.T) {
	dir := t.TempDir()
	cfg := Tenant{
		ID:       "acme-test",
		Name:     "  Acme Test  ",
		Services: []string{" repair ", "REPAIR", "installation"},
		Windows:  []string{"evening", "morning", "morning", "invalid"},
		Persona:  Persona{Languages: []string{"en", "en", "hi"}},
	}
	saved, err := Save(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != "Acme Test" || len(saved.Services) != 2 || len(saved.Windows) != 2 {
		t.Fatalf("saved = %+v", saved)
	}
	if saved.Persona.AgentName != "Alex" || saved.Persona.Tone == "" {
		t.Fatalf("persona defaults = %+v", saved.Persona)
	}
	reg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, ok := reg.Get("acme-test")
	if !ok || loaded.Name != "Acme Test" || len(loaded.Services) != 2 {
		t.Fatalf("loaded = %+v ok=%v", loaded, ok)
	}
	if _, err := os.Stat(filepath.Join(dir, "acme-test", "config.json")); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRejectsInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	for _, cfg := range []Tenant{
		{ID: "a", Name: "", Services: []string{"repair"}, Windows: []string{"morning"}},
		{ID: "b", Name: "B", Services: nil, Windows: []string{"morning"}},
		{ID: "c", Name: "C", Services: []string{"repair"}, Windows: []string{"noon"}},
	} {
		if _, err := Save(dir, cfg); err == nil {
			t.Fatalf("invalid config accepted: %+v", cfg)
		}
	}
}
