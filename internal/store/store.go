package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chinmay/switchboard/internal/escalate"
)

type Record struct {
	ID        string          `json:"id"`
	Tenant    string          `json:"tenant"`
	Lang      string          `json:"lang"`
	Turns     int             `json:"turns"`
	Escalated bool            `json:"escalated"`
	Reason    string          `json:"reason"`
	Signal    escalate.Signal `json:"signal"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   time.Time       `json:"ended_at"`
	Label     *bool           `json:"label,omitempty"`
}

type Store struct {
	mu     sync.Mutex
	dir    string
	max    int
	order  []string
	byID   map[string]*Record
	labels map[string]bool
}

func New(dir string, max int) *Store {
	s := &Store{dir: dir, max: max, byID: map[string]*Record{}, labels: map[string]bool{}}
	os.MkdirAll(dir, 0o755)
	s.load()
	return s
}

func (s *Store) Save(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[r.ID] = &r
	s.order = append(s.order, r.ID)
	s.trim()
	s.appendJSONL(r)
}

func (s *Store) SetLabel(id string, label bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return false
	}
	s.labels[id] = label
	if rec, ok := s.byID[id]; ok {
		l := label
		rec.Label = &l
	}
	s.writeLabels()
	return true
}

func (s *Store) List() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, 0, len(s.order))
	for i := len(s.order) - 1; i >= 0; i-- {
		if r, ok := s.byID[s.order[i]]; ok {
			rec := *r
			if l, ok := s.labels[rec.ID]; ok {
				lb := l
				rec.Label = &lb
			}
			out = append(out, rec)
		}
	}
	return out
}

func (s *Store) Labels() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]bool, len(s.labels))
	for k, v := range s.labels {
		out[k] = v
	}
	return out
}

func (s *Store) trim() {
	for len(s.order) > s.max {
		old := s.order[0]
		s.order = s.order[1:]
		delete(s.byID, old)
	}
}

func (s *Store) appendJSONL(r Record) {
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(s.dir, "calls.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(b, '\n'))
}

func (s *Store) load() {
	f, err := os.Open(filepath.Join(s.dir, "calls.jsonl"))
	if err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var r Record
			if json.Unmarshal(sc.Bytes(), &r) == nil {
				s.byID[r.ID] = &r
				s.order = append(s.order, r.ID)
			}
		}
		f.Close()
		s.trim()
	}

	if b, err := os.ReadFile(filepath.Join(s.dir, "labels.json")); err == nil {
		json.Unmarshal(b, &s.labels)
	}
}

func (s *Store) writeLabels() {
	b, err := json.Marshal(s.labels)
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(s.dir, "labels.json"), b, 0o644)
}
