package jobs

import "sync"

type Registry struct {
	mu sync.Mutex
	m  map[string]*Store
}

func NewRegistry() *Registry {
	return &Registry{m: map[string]*Store{}}
}

func (r *Registry) For(tenant string) *Store {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.m[tenant]; ok {
		return s
	}
	s := NewStore()
	r.m[tenant] = s
	return s
}
