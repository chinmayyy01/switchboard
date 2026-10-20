package ws

import "sync"

type bridge struct {
	mu         sync.Mutex
	caller     *connWriter
	supervisor *connWriter
}

func (b *bridge) setCaller(w *connWriter) {
	b.mu.Lock()
	b.caller = w
	b.mu.Unlock()
}

func (b *bridge) clearCaller(w *connWriter) {
	b.mu.Lock()
	if b.caller == w {
		b.caller = nil
	}
	b.mu.Unlock()
}

func (b *bridge) setSupervisor(w *connWriter) {
	b.mu.Lock()
	b.supervisor = w
	b.mu.Unlock()
}

func (b *bridge) clearSupervisor(w *connWriter) {
	b.mu.Lock()
	if b.supervisor == w {
		b.supervisor = nil
	}
	b.mu.Unlock()
}

func (b *bridge) hasSupervisor() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.supervisor != nil
}

func (b *bridge) toSupervisor(pcm []int16) {
	b.mu.Lock()
	s := b.supervisor
	b.mu.Unlock()
	if s != nil {
		s.writePCM(pcm)
	}
}

func (b *bridge) toCaller(pcm []int16) {
	b.mu.Lock()
	c := b.caller
	b.mu.Unlock()
	if c != nil {
		c.writePCM(pcm)
	}
}

type BridgeRegistry struct {
	mu sync.Mutex
	m  map[string]*bridge
}

func NewBridgeRegistry() *BridgeRegistry {
	return &BridgeRegistry{m: map[string]*bridge{}}
}

func (r *BridgeRegistry) getOrCreate(id string) *bridge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.m[id]; ok {
		return b
	}
	b := &bridge{}
	r.m[id] = b
	return b
}

func (r *BridgeRegistry) get(id string) (*bridge, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.m[id]
	return b, ok
}

func (r *BridgeRegistry) remove(id string) {
	r.mu.Lock()
	delete(r.m, id)
	r.mu.Unlock()
}
