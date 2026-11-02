package jobs

import (
	"path/filepath"
	"sync"

	"github.com/chinmay/switchboard/internal/staff"
)

type Registry struct {
	mu       sync.Mutex
	m        map[string]*Store
	staffDir string
}

func NewRegistry(staffDir string) *Registry {
	return &Registry{m: map[string]*Store{}, staffDir: staffDir}
}

func (r *Registry) For(tenant string) *Store {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.m[tenant]; ok {
		return s
	}
	s := NewStore()
	if r.staffDir != "" {
		if sr, err := staff.Load(filepath.Join(r.staffDir, tenant, "staff.json")); err == nil {
			s.Staff = sr
		}
	}
	r.m[tenant] = s
	return s
}
