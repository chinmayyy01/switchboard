package jobs

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/chinmay/switchboard/internal/staff"
)

type Registry struct {
	mu       sync.Mutex
	m        map[string]TenantStore
	staffDir string
	db       *sql.DB
}

func NewRegistry(staffDir string) *Registry {
	return &Registry{m: map[string]TenantStore{}, staffDir: staffDir}
}

func NewPostgresRegistry(ctx context.Context, db *sql.DB, staffDir string, tenantIDs []string) (*Registry, error) {
	r := &Registry{m: map[string]TenantStore{}, staffDir: staffDir, db: db}
	for _, tenantID := range tenantIDs {
		if staffDir == "" {
			continue
		}
		registry, err := staff.Load(filepath.Join(staffDir, tenantID, "staff.json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := NewPostgresStore(db, tenantID).EnsureTenantStaff(ctx, registry.Employees()); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) For(tenant string) TenantStore {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.m[tenant]; ok {
		return s
	}
	var s TenantStore
	if r.db != nil {
		s = NewPostgresStore(r.db, tenant)
	} else {
		memory := NewStore()
		if r.staffDir != "" {
			if sr, err := staff.Load(filepath.Join(r.staffDir, tenant, "staff.json")); err == nil {
				memory.Staff = sr
			}
		}
		s = memory
	}
	r.m[tenant] = s
	return s
}
