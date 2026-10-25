package jobs

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalid        = errors.New("invalid request")
	ErrNoAvailability = errors.New("no availability")
	ErrNotFound       = errors.New("job not found")
)

var Windows = []string{"morning", "afternoon", "evening"}

type Job struct {
	ID      string `json:"id"`
	Service string `json:"service"`
	Address string `json:"address"`
	Date    string `json:"date"`
	Window  string `json:"window"`
	Phone   string `json:"phone"`
	Status  string `json:"status"`
}

type Store struct {
	mu       sync.Mutex
	jobs     map[string]*Job
	counts   map[string]int
	capacity int
	seq      int
	Now      func() time.Time
}

func NewStore() *Store {
	return &Store{
		jobs:     map[string]*Job{},
		counts:   map[string]int{},
		capacity: 2,
		Now:      time.Now,
	}
}

func (s *Store) Availability(date string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []string{}
	for _, w := range Windows {
		if s.counts[date+"|"+w] < s.capacity {
			out = append(out, w)
		}
	}
	return out
}

func (s *Store) Book(service, date, window, address, phone string) (Job, error) {
	service = strings.TrimSpace(service)
	address = strings.TrimSpace(address)
	if service == "" || address == "" || date == "" || !ValidWindow(window) {
		return Job{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := date + "|" + window
	if s.counts[key] >= s.capacity {
		return Job{}, ErrNoAvailability
	}
	s.seq++
	j := &Job{
		ID:      fmt.Sprintf("job-%03d", s.seq),
		Service: service,
		Address: address,
		Date:    date,
		Window:  window,
		Phone:   NormalizePhone(phone),
		Status:  "booked",
	}
	s.jobs[j.ID] = j
	s.counts[key]++
	return *j, nil
}

func (s *Store) ByPhone(phone string) []Job {
	p := NormalizePhone(phone)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Job{}
	for _, j := range s.jobs {
		if j.Phone == p && j.Status == "booked" {
			out = append(out, *j)
		}
	}
	return out
}

func (s *Store) Get(id string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *j, true
}

func (s *Store) Reschedule(id, date, window string) error {
	if !ValidWindow(window) || date == "" {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok || j.Status != "booked" {
		return ErrNotFound
	}
	key := date + "|" + window
	if s.counts[key] >= s.capacity {
		return ErrNoAvailability
	}
	old := j.Date + "|" + j.Window
	if s.counts[old] > 0 {
		s.counts[old]--
	}
	j.Date = date
	j.Window = window
	s.counts[key]++
	return nil
}

func (s *Store) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok || j.Status != "booked" {
		return ErrNotFound
	}
	key := j.Date + "|" + j.Window
	if s.counts[key] > 0 {
		s.counts[key]--
	}
	j.Status = "cancelled"
	return nil
}

func ValidWindow(w string) bool {
	for _, x := range Windows {
		if x == w {
			return true
		}
	}
	return false
}

func NormalizePhone(p string) string {
	var b strings.Builder
	for _, r := range p {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
