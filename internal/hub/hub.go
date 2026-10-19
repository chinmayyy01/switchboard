package hub

import (
	"sync"
	"time"
)

type Turn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type Call struct {
	ID         string    `json:"id"`
	Lang       string    `json:"lang"`
	State      string    `json:"state"`
	LastText   string    `json:"last_text"`
	Escalated  bool      `json:"escalated"`
	Reason     string    `json:"reason"`
	TakenOver  bool      `json:"taken_over"`
	Transcript []Turn    `json:"transcript"`
	StartedAt  time.Time `json:"started_at"`
}

type Hub struct {
	mu       sync.Mutex
	calls    map[string]*Call
	subs     map[int]chan []Call
	controls map[string]func()
	nextID   int
}

func New() *Hub {
	return &Hub{
		calls:    map[string]*Call{},
		subs:     map[int]chan []Call{},
		controls: map[string]func(){},
	}
}

func (h *Hub) Register(id string) {
	h.mu.Lock()
	h.calls[id] = &Call{ID: id, StartedAt: time.Now()}
	h.publishLocked()
	h.mu.Unlock()
}

func (h *Hub) Unregister(id string) {
	h.mu.Lock()
	delete(h.calls, id)
	delete(h.controls, id)
	h.publishLocked()
	h.mu.Unlock()
}

func (h *Hub) SetControl(id string, fn func()) {
	h.mu.Lock()
	h.controls[id] = fn
	h.mu.Unlock()
}

func (h *Hub) SetLang(id, lang string)   { h.mutate(id, func(c *Call) { c.Lang = lang }) }
func (h *Hub) SetState(id, state string) { h.mutate(id, func(c *Call) { c.State = state }) }
func (h *Hub) SetText(id, text string)   { h.mutate(id, func(c *Call) { c.LastText = text }) }
func (h *Hub) SetReason(id, reason string) {
	h.mutate(id, func(c *Call) { c.Reason = reason })
}
func (h *Hub) SetEscalated(id string, v bool) {
	h.mutate(id, func(c *Call) { c.Escalated = v })
}

func (h *Hub) AddTurn(id, role, text string) {
	h.mutate(id, func(c *Call) {
		c.Transcript = append(c.Transcript, Turn{Role: role, Text: text})
	})
}

func (h *Hub) TakeOver(id string) bool {
	h.mu.Lock()
	fn, ok := h.controls[id]
	if ok {
		if c, exists := h.calls[id]; exists {
			c.TakenOver = true
		}
	}
	h.publishLocked()
	h.mu.Unlock()
	if ok {
		fn()
	}
	return ok
}

func (h *Hub) mutate(id string, fn func(*Call)) {
	h.mu.Lock()
	if c, ok := h.calls[id]; ok {
		fn(c)
		h.publishLocked()
	}
	h.mu.Unlock()
}

func (h *Hub) Subscribe() (int, <-chan []Call) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	ch := make(chan []Call, 1)
	ch <- h.snapshotLocked()
	h.subs[id] = ch
	return id, ch
}

func (h *Hub) Unsubscribe(id int) {
	h.mu.Lock()
	delete(h.subs, id)
	h.mu.Unlock()
}

func (h *Hub) publishLocked() {
	snap := h.snapshotLocked()
	for _, ch := range h.subs {
		select {
		case ch <- snap:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- snap:
			default:
			}
		}
	}
}

func (h *Hub) snapshotLocked() []Call {
	out := make([]Call, 0, len(h.calls))
	for _, c := range h.calls {
		out = append(out, *c)
	}
	return out
}
