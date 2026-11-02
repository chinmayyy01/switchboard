package jobs

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chinmay/switchboard/internal/staff"
)

var (
	ErrInvalid        = errors.New("invalid request")
	ErrNoAvailability = errors.New("no availability")
	ErrNoStaff        = errors.New("no employee available")
	ErrNotFound       = errors.New("job not found")
)

var Windows = []string{"morning", "afternoon", "evening"}

type Job struct {
	ID           string `json:"id"`
	Service      string `json:"service"`
	Address      string `json:"address"`
	Date         string `json:"date"`
	Window       string `json:"window"`
	Phone        string `json:"phone"`
	Status       string `json:"status"`
	EmployeeID   string `json:"employee_id,omitempty"`
	EmployeeName string `json:"employee_name,omitempty"`
}

type Slot struct {
	Date   string `json:"date"`
	Window string `json:"window"`
}

type Store struct {
	mu       sync.Mutex
	jobs     map[string]*Job
	counts   map[string]int
	capacity int
	seq      int
	windows  []string
	Now      func() time.Time
	Staff    *staff.Registry
}

func NewStore() *Store {
	return &Store{
		jobs:     map[string]*Job{},
		counts:   map[string]int{},
		capacity: 2,
		windows:  append([]string{}, Windows...),
		Now:      time.Now,
	}
}

func (s *Store) SetWindows(windows []string) {
	if len(windows) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.windows = append([]string{}, windows...)
}

func (s *Store) SetStaff(r *staff.Registry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Staff = r
}

func (s *Store) Availability(date string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.availabilityLocked(date)
}

func (s *Store) availabilityLocked(date string) []string {
	out := []string{}
	for _, w := range s.windows {
		if s.counts[date+"|"+w] < s.capacity {
			out = append(out, w)
		}
	}
	return out
}

func (s *Store) AvailabilityFor(service, date string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Staff == nil || strings.TrimSpace(service) == "" {
		return s.availabilityLocked(date)
	}
	out := []string{}
	for _, w := range s.windows {
		if len(s.Staff.Available(service, date, w)) > 0 {
			out = append(out, w)
		}
	}
	return out
}

func (s *Store) Alternatives(service, from string, days int) []Slot {
	s.mu.Lock()
	defer s.mu.Unlock()

	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		now := time.Now
		if s.Now != nil {
			now = s.Now
		}
		start = now()
	}
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)

	out := []Slot{}
	for d := 0; d <= days && len(out) < 6; d++ {
		date := start.AddDate(0, 0, d).Format("2006-01-02")
		for _, w := range s.windows {
			ok := false
			if s.Staff != nil && strings.TrimSpace(service) != "" {
				ok = len(s.Staff.Available(service, date, w)) > 0
			} else {
				ok = s.counts[date+"|"+w] < s.capacity
			}
			if ok {
				out = append(out, Slot{Date: date, Window: w})
				if len(out) >= 6 {
					break
				}
			}
		}
	}
	return out
}

func (s *Store) Book(service, date, window, address, phone string) (Job, error) {
	service = strings.TrimSpace(service)
	address = strings.TrimSpace(address)
	if service == "" || address == "" || date == "" {
		return Job{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.validWindowLocked(window) {
		return Job{}, ErrInvalid
	}

	var emp staff.Employee
	if s.Staff != nil {
		e, ok := s.Staff.Assign(service, date, window)
		if !ok {
			return Job{}, ErrNoStaff
		}
		emp = e
	} else {
		key := date + "|" + window
		if s.counts[key] >= s.capacity {
			return Job{}, ErrNoAvailability
		}
	}

	s.seq++
	j := &Job{
		ID:           fmt.Sprintf("job-%03d", s.seq),
		Service:      service,
		Address:      address,
		Date:         date,
		Window:       window,
		Phone:        NormalizePhone(phone),
		Status:       "booked",
		EmployeeID:   emp.ID,
		EmployeeName: emp.Name,
	}
	s.jobs[j.ID] = j
	s.counts[date+"|"+window]++
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
	if date == "" {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok || j.Status != "booked" {
		return ErrNotFound
	}
	if !s.validWindowLocked(window) {
		return ErrInvalid
	}
	if j.Date == date && j.Window == window {
		return nil
	}

	var emp staff.Employee
	if s.Staff != nil {
		e, ok := s.Staff.Assign(j.Service, date, window)
		if !ok {
			return ErrNoStaff
		}
		emp = e
	} else if s.counts[date+"|"+window] >= s.capacity {
		return ErrNoAvailability
	}

	old := j.Date + "|" + j.Window
	if s.counts[old] > 0 {
		s.counts[old]--
	}
	if s.Staff != nil {
		s.Staff.Release(j.EmployeeID, j.Date, j.Window)
	}
	j.Date = date
	j.Window = window
	j.EmployeeID = emp.ID
	j.EmployeeName = emp.Name
	s.counts[date+"|"+window]++
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
	if s.Staff != nil {
		s.Staff.Release(j.EmployeeID, j.Date, j.Window)
	}
	j.Status = "cancelled"
	return nil
}

func (s *Store) validWindowLocked(w string) bool {
	for _, x := range s.windows {
		if x == w {
			return true
		}
	}
	return false
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
