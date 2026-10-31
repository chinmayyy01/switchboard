package tenant

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Tenant struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Services []string `json:"services"`
	Windows  []string `json:"windows"`
}

type Registry struct {
	byID map[string]Tenant
}

func Load(dir string) (*Registry, error) {
	reg := &Registry{byID: map[string]Tenant{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "config.json")
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var t Tenant
		if err := json.Unmarshal(b, &t); err != nil {
			return nil, fmt.Errorf("tenant %s: %w", entry.Name(), err)
		}
		if t.ID == "" {
			t.ID = entry.Name()
		}
		reg.byID[t.ID] = t
	}
	return reg, nil
}

func (r *Registry) Get(id string) (Tenant, bool) {
	t, ok := r.byID[id]
	return t, ok
}

func (r *Registry) IDs() []string {
	out := make([]string, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func Default() *Registry {
	return &Registry{byID: map[string]Tenant{
		"rivera": {
			ID:       "rivera",
			Name:     "Rivera Plumbing & HVAC",
			Services: []string{"leak repair", "drain cleaning", "water heater", "ac service", "heating"},
			Windows:  []string{"morning", "afternoon", "evening"},
		},
	}}
}
