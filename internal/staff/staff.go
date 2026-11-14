package staff

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	RoleTechnician = "technician"
	RoleDispatcher = "dispatcher"
	RoleManager    = "manager"
	RoleOther      = "other"
)

var ErrInvalidEmployee = errors.New("invalid employee")

type Employee struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Role   string   `json:"role"`
	Email  string   `json:"email,omitempty"`
	Phone  string   `json:"phone,omitempty"`
	Skills []string `json:"skills"`
	Days   []string `json:"days"`
	Shifts []string `json:"shifts"`
	Active bool     `json:"active"`
}

type Registry struct {
	mu        sync.Mutex
	employees []Employee
	busy      map[string]map[string]bool
}

func Load(path string) (*Registry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file struct {
		Employees []Employee `json:"employees"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		return nil, err
	}
	return New(file.Employees), nil
}

func New(employees []Employee) *Registry {
	copyEmployees := make([]Employee, 0, len(employees))
	for _, employee := range employees {
		// Existing tenant staff files predate role and active fields.
		if employee.Role == "" {
			employee.Role = RoleTechnician
			employee.Active = true
		}
		copyEmployees = append(copyEmployees, cloneEmployee(employee))
	}
	return &Registry{employees: copyEmployees, busy: map[string]map[string]bool{}}
}

func Normalize(employee Employee) (Employee, error) {
	employee.Name = strings.TrimSpace(employee.Name)
	employee.Role = strings.ToLower(strings.TrimSpace(employee.Role))
	if employee.Role == "" {
		employee.Role = RoleTechnician
	}
	switch employee.Role {
	case RoleTechnician, RoleDispatcher, RoleManager, RoleOther:
	default:
		return Employee{}, ErrInvalidEmployee
	}
	employee.Email = strings.ToLower(strings.TrimSpace(employee.Email))
	employee.Phone = strings.TrimSpace(employee.Phone)
	employee.Skills = normalizeList(employee.Skills, true)
	employee.Days = normalizeList(employee.Days, true)
	employee.Shifts = normalizeList(employee.Shifts, true)
	if employee.Name == "" {
		return Employee{}, ErrInvalidEmployee
	}
	if employee.Role == RoleTechnician && len(employee.Skills) == 0 {
		return Employee{}, ErrInvalidEmployee
	}
	for _, day := range employee.Days {
		switch day {
		case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
		default:
			return Employee{}, ErrInvalidEmployee
		}
	}
	for _, shift := range employee.Shifts {
		switch shift {
		case "morning", "afternoon", "evening":
		default:
			return Employee{}, ErrInvalidEmployee
		}
	}
	return employee, nil
}

func normalizeList(values []string, lower bool) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (r *Registry) Employees() []Employee {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Employee, len(r.employees))
	for i, employee := range r.employees {
		out[i] = cloneEmployee(employee)
	}
	return out
}

func (r *Registry) Upsert(employee Employee) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, existing := range r.employees {
		if existing.ID == employee.ID {
			r.employees[i] = cloneEmployee(employee)
			return
		}
	}
	r.employees = append(r.employees, cloneEmployee(employee))
}

func (r *Registry) SetActive(id string, active bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.employees {
		if r.employees[i].ID == id {
			r.employees[i].Active = active
			return true
		}
	}
	return false
}

func (r *Registry) Available(service, date, window string) []Employee {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.availableLocked(service, date, window)
}

func (r *Registry) availableLocked(service, date, window string) []Employee {
	wd, ok := parseDate(date)
	if !ok {
		return nil
	}
	key := date + "|" + window
	out := []Employee{}
	for _, e := range r.employees {
		if !e.Active || e.Role != RoleTechnician || !worksDay(e, wd) || !worksShift(e, window) || !hasSkill(e, service) {
			continue
		}
		if r.busy[e.ID][key] {
			continue
		}
		out = append(out, e)
	}
	return out
}

func cloneEmployee(employee Employee) Employee {
	employee.Skills = append([]string(nil), employee.Skills...)
	employee.Days = append([]string(nil), employee.Days...)
	employee.Shifts = append([]string(nil), employee.Shifts...)
	return employee
}

func (r *Registry) Assign(service, date, window string) (Employee, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	avail := r.availableLocked(service, date, window)
	if len(avail) == 0 {
		return Employee{}, false
	}
	e := avail[0]
	if r.busy[e.ID] == nil {
		r.busy[e.ID] = map[string]bool{}
	}
	r.busy[e.ID][date+"|"+window] = true
	return e, true
}

func (r *Registry) Release(employeeID, date, window string) {
	if employeeID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.busy[employeeID], date+"|"+window)
}

func (r *Registry) Busy() map[string][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string][]string, len(r.busy))
	for id, slots := range r.busy {
		list := make([]string, 0, len(slots))
		for slot := range slots {
			list = append(list, slot)
		}
		sort.Strings(list)
		out[id] = list
	}
	return out
}

func parseDate(s string) (time.Weekday, bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, false
	}
	return t.Weekday(), true
}

func worksDay(e Employee, wd time.Weekday) bool {
	if len(e.Days) == 0 {
		return true
	}
	name := strings.ToLower(wd.String())
	for _, d := range e.Days {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if d == name || (len(d) >= 3 && strings.HasPrefix(name, d)) {
			return true
		}
	}
	return false
}

func worksShift(e Employee, window string) bool {
	if len(e.Shifts) == 0 {
		return true
	}
	for _, s := range e.Shifts {
		if strings.EqualFold(strings.TrimSpace(s), window) {
			return true
		}
	}
	return false
}

func hasSkill(e Employee, service string) bool {
	if strings.TrimSpace(service) == "" {
		return true
	}
	for _, s := range e.Skills {
		if skillMatch(s, service) {
			return true
		}
	}
	return false
}

func skillMatch(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}
