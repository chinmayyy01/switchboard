package tenant

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Telephony struct {
	Provider string `json:"provider,omitempty"`
	Number   string `json:"number,omitempty"`
}

type Persona struct {
	AgentName string   `json:"agent_name,omitempty"`
	Tone      string   `json:"tone,omitempty"`
	Languages []string `json:"languages,omitempty"`
	Greeting  string   `json:"greeting,omitempty"`
}

type Tenant struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Services      []string  `json:"services"`
	Windows       []string  `json:"windows"`
	Telephony     Telephony `json:"telephony,omitempty"`
	Persona       Persona   `json:"persona,omitempty"`
	ManagerNumber string    `json:"manager_number,omitempty"`
	Disclosure    string    `json:"disclosure,omitempty"`
}

type Registry struct {
	byID map[string]Tenant
}

// New builds a registry from explicit tenants. Used by tests and by callers
// that load configuration from somewhere other than the tenant directory.
func New(items ...Tenant) *Registry {
	reg := &Registry{byID: make(map[string]Tenant, len(items))}
	for _, item := range items {
		reg.byID[item.ID] = item
	}
	return reg
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
		if t.ManagerNumber == "" {
			t.ManagerNumber = os.Getenv("MANAGER_NUMBER_" + strings.ToUpper(t.ID))
			if t.ManagerNumber == "" {
				t.ManagerNumber = os.Getenv("MANAGER_NUMBER")
			}
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

// ManagerPhone returns the tenant's configured transfer target.
func (t Tenant) ManagerPhone() string {
	return t.ManagerNumber
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
