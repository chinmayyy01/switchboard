package jobs

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chinmay/switchboard/internal/staff"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrations embed.FS

type PostgresStore struct {
	db      *sql.DB
	tenant  string
	mu      sync.RWMutex
	windows []string
}

func OpenPostgres(ctx context.Context, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("DATABASE_URL is required for durable bookings")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to bookings database: %w", err)
	}
	if err := MigratePostgres(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate bookings database: %w", err)
	}
	return db, nil
}

func MigratePostgres(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"SELECT pg_advisory_xact_lock(hashtextextended('switchboard_schema_migrations', 0))",
		); err != nil {
			tx.Rollback()
			return err
		}
		var applied bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)",
			version,
		).Scan(&applied); err != nil {
			tx.Rollback()
			return err
		}
		if applied {
			tx.Rollback()
			continue
		}
		contents, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations(version) VALUES ($1)", version,
		); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func NewPostgresStore(db *sql.DB, tenant string) *PostgresStore {
	return &PostgresStore{
		db:      db,
		tenant:  tenant,
		windows: append([]string(nil), Windows...),
	}
}

func (s *PostgresStore) SetWindows(windows []string) {
	if len(windows) == 0 {
		return
	}
	s.mu.Lock()
	s.windows = append([]string(nil), windows...)
	s.mu.Unlock()
}

func (s *PostgresStore) CurrentTime() time.Time { return time.Now() }

func (s *PostgresStore) validWindow(window string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, candidate := range s.windows {
		if candidate == window {
			return true
		}
	}
	return false
}

func (s *PostgresStore) AvailabilityContext(ctx context.Context, service, date string) ([]string, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, ErrInvalid
	}
	s.mu.RLock()
	windows := append([]string(nil), s.windows...)
	s.mu.RUnlock()
	available := make([]string, 0, len(windows))
	for _, window := range windows {
		employees, err := s.availableEmployees(ctx, nil, service, date, window, "", false)
		if err != nil {
			return nil, err
		}
		if len(employees) > 0 {
			available = append(available, window)
		}
	}
	return available, nil
}

func (s *PostgresStore) AlternativesContext(ctx context.Context, service, from string, days int) ([]Slot, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		start = s.CurrentTime()
	}
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	s.mu.RLock()
	windows := append([]string(nil), s.windows...)
	s.mu.RUnlock()
	out := make([]Slot, 0, 6)
	for offset := 0; offset <= days && len(out) < 6; offset++ {
		date := start.AddDate(0, 0, offset).Format("2006-01-02")
		for _, window := range windows {
			employees, err := s.availableEmployees(ctx, nil, service, date, window, "", false)
			if err != nil {
				return nil, err
			}
			if len(employees) > 0 {
				out = append(out, Slot{Date: date, Window: window})
			}
			if len(out) >= 6 {
				break
			}
		}
	}
	return out, nil
}

func (s *PostgresStore) BookContext(ctx context.Context, request BookingRequest) (Job, error) {
	request.Service = strings.TrimSpace(request.Service)
	request.Address = strings.TrimSpace(request.Address)
	request.Phone = NormalizePhone(request.Phone)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = newID("manual_")
	}
	if request.Service == "" || request.Address == "" || request.Date == "" {
		return Job{}, ErrInvalid
	}
	if _, err := time.Parse("2006-01-02", request.Date); err != nil || !s.validWindow(request.Window) {
		return Job{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	// Serialize retries of the same logical call before checking the unique key.
	if _, err := tx.ExecContext(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, 0))",
		s.tenant+"|"+request.IdempotencyKey,
	); err != nil {
		return Job{}, err
	}
	if existing, found, err := s.getByIdempotencyKey(ctx, tx, request.IdempotencyKey); err != nil {
		return Job{}, err
	} else if found {
		if err := tx.Commit(); err != nil {
			return Job{}, err
		}
		return existing, nil
	}

	candidates, err := s.availableEmployees(ctx, tx, request.Service, request.Date, request.Window, "", true)
	if err != nil {
		return Job{}, err
	}
	for _, employee := range candidates {
		job := Job{
			ID:           newID("job_"),
			Service:      request.Service,
			Address:      request.Address,
			Date:         request.Date,
			Window:       request.Window,
			Phone:        request.Phone,
			Status:       "booked",
			EmployeeID:   employee.ID,
			EmployeeName: employee.Name,
		}
		err := tx.QueryRowContext(ctx, `
INSERT INTO jobs
    (id, tenant_id, idempotency_key, service, address, service_date, time_window, phone, status, employee_id, employee_name)
VALUES ($1, $2, $3, $4, $5, $6::date, $7, $8, 'booked', $9, $10)
ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
RETURNING id`,
			job.ID, s.tenant, request.IdempotencyKey, job.Service, job.Address,
			job.Date, job.Window, job.Phone, job.EmployeeID, job.EmployeeName,
		).Scan(&job.ID)
		if err == nil {
			if err := tx.Commit(); err != nil {
				return Job{}, err
			}
			return job, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "jobs_employee_slot_unique" {
			continue
		}
		if errors.Is(err, sql.ErrNoRows) {
			if existing, found, lookupErr := s.getByIdempotencyKey(ctx, tx, request.IdempotencyKey); lookupErr != nil {
				return Job{}, lookupErr
			} else if found {
				if err := tx.Commit(); err != nil {
					return Job{}, err
				}
				return existing, nil
			}
			continue
		}
		return Job{}, err
	}
	return Job{}, ErrNoStaff
}

func (s *PostgresStore) availableEmployees(ctx context.Context, tx *sql.Tx, service, date, window, exceptJobID string, lock bool) ([]staff.Employee, error) {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, ErrInvalid
	}
	day := strings.ToLower(parsed.Weekday().String()[:3])
	query := `
SELECT e.id, e.name, e.role, e.email, e.phone, e.skills, e.days, e.shifts, e.active
FROM employees e
WHERE e.tenant_id = $1
  AND e.active = TRUE
  AND e.role = 'technician'
  AND (e.days = '[]'::jsonb OR e.days ? $2)
  AND (e.shifts = '[]'::jsonb OR e.shifts ? $3)
  AND EXISTS (
      SELECT 1 FROM jsonb_array_elements_text(e.skills) AS skill_value(skill)
      WHERE lower(skill) = lower($4)
         OR position(lower(skill) in lower($4)) > 0
         OR position(lower($4) in lower(skill)) > 0
  )
  AND NOT EXISTS (
      SELECT 1 FROM jobs j
      WHERE j.tenant_id = e.tenant_id
        AND j.employee_id = e.id
        AND j.status = 'booked'
        AND j.service_date = $5::date
        AND j.time_window = $3
        AND j.id <> $6
  )
ORDER BY e.id`
	if lock {
		query += " FOR UPDATE OF e SKIP LOCKED"
	}
	var rows *sql.Rows
	if tx != nil {
		rows, err = tx.QueryContext(ctx, query, s.tenant, day, window, service, date, exceptJobID)
	} else {
		rows, err = s.db.QueryContext(ctx, query, s.tenant, day, window, service, date, exceptJobID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]staff.Employee, 0)
	for rows.Next() {
		employee, err := scanEmployee(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, employee)
	}
	return out, rows.Err()
}

func (s *PostgresStore) getByIdempotencyKey(ctx context.Context, tx *sql.Tx, key string) (Job, bool, error) {
	job, err := scanJob(tx.QueryRowContext(ctx, `
SELECT id, service, address, to_char(service_date, 'YYYY-MM-DD'), time_window,
       phone, status, COALESCE(employee_id, ''), employee_name
FROM jobs WHERE tenant_id = $1 AND idempotency_key = $2`, s.tenant, key))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	return job, err == nil, err
}

func (s *PostgresStore) ByPhoneContext(ctx context.Context, phone string) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, service, address, to_char(service_date, 'YYYY-MM-DD'), time_window,
       phone, status, COALESCE(employee_id, ''), employee_name
FROM jobs WHERE tenant_id = $1 AND phone = $2 AND status = 'booked'
ORDER BY service_date, time_window, created_at`, s.tenant, NormalizePhone(phone))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (s *PostgresStore) GetContext(ctx context.Context, id string) (Job, bool, error) {
	job, err := scanJob(s.db.QueryRowContext(ctx, `
SELECT id, service, address, to_char(service_date, 'YYYY-MM-DD'), time_window,
       phone, status, COALESCE(employee_id, ''), employee_name
FROM jobs WHERE tenant_id = $1 AND id = $2`, s.tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	return job, err == nil, err
}

func (s *PostgresStore) RescheduleContext(ctx context.Context, id, date, window string) error {
	if date == "" || !s.validWindow(window) {
		return ErrInvalid
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	job, err := scanJob(tx.QueryRowContext(ctx, `
SELECT id, service, address, to_char(service_date, 'YYYY-MM-DD'), time_window,
       phone, status, COALESCE(employee_id, ''), employee_name
FROM jobs WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, s.tenant, id))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && job.Status != "booked") {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if job.Date == date && job.Window == window {
		return tx.Commit()
	}
	candidates, err := s.availableEmployees(ctx, tx, job.Service, date, window, job.ID, true)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return ErrNoStaff
	}
	for _, employee := range candidates {
		result, err := tx.ExecContext(ctx, `
UPDATE jobs SET service_date = $3::date, time_window = $4,
       employee_id = $5, employee_name = $6, updated_at = NOW()
WHERE tenant_id = $1 AND id = $2 AND status = 'booked'`,
			s.tenant, id, date, window, employee.ID, employee.Name)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "jobs_employee_slot_unique" {
				continue
			}
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count == 1 {
			return tx.Commit()
		}
	}
	return ErrNoStaff
}

func (s *PostgresStore) CancelContext(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE jobs SET status = 'cancelled', updated_at = NOW()
WHERE tenant_id = $1 AND id = $2 AND status = 'booked'`, s.tenant, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) EmployeesContext(ctx context.Context) ([]EmployeeWithJobs, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, role, email, phone, skills, days, shifts, active
FROM employees WHERE tenant_id = $1 ORDER BY lower(name), id`, s.tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EmployeeWithJobs, 0)
	indices := make(map[string]int)
	for rows.Next() {
		employee, err := scanEmployee(rows)
		if err != nil {
			return nil, err
		}
		indices[employee.ID] = len(out)
		out = append(out, EmployeeWithJobs{Employee: employee, Busy: []string{}})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	busyRows, err := s.db.QueryContext(ctx, `
SELECT employee_id, to_char(service_date, 'YYYY-MM-DD'), time_window
FROM jobs WHERE tenant_id = $1 AND status = 'booked' AND employee_id IS NOT NULL
  AND service_date >= CURRENT_DATE
ORDER BY service_date, time_window`, s.tenant)
	if err != nil {
		return nil, err
	}
	defer busyRows.Close()
	for busyRows.Next() {
		var employeeID, date, window string
		if err := busyRows.Scan(&employeeID, &date, &window); err != nil {
			return nil, err
		}
		if i, ok := indices[employeeID]; ok {
			out[i].Busy = append(out[i].Busy, date+"|"+window)
		}
	}
	return out, busyRows.Err()
}

func (s *PostgresStore) SaveEmployeeContext(ctx context.Context, employee staff.Employee) (staff.Employee, error) {
	employee, err := staff.Normalize(employee)
	if err != nil {
		return staff.Employee{}, err
	}
	if employee.ID == "" {
		employee.ID = newID("emp_")
	}
	skills, err := json.Marshal(employee.Skills)
	if err != nil {
		return staff.Employee{}, err
	}
	days, err := json.Marshal(employee.Days)
	if err != nil {
		return staff.Employee{}, err
	}
	shifts, err := json.Marshal(employee.Shifts)
	if err != nil {
		return staff.Employee{}, err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO employees
    (tenant_id, id, name, role, email, phone, skills, days, shifts, active, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
ON CONFLICT (tenant_id, id) DO UPDATE SET
    name = EXCLUDED.name,
    role = EXCLUDED.role,
    email = EXCLUDED.email,
    phone = EXCLUDED.phone,
    skills = EXCLUDED.skills,
    days = EXCLUDED.days,
    shifts = EXCLUDED.shifts,
    active = EXCLUDED.active,
    updated_at = NOW()`,
		s.tenant, employee.ID, employee.Name, employee.Role, employee.Email, employee.Phone,
		skills, days, shifts, employee.Active)
	if err != nil {
		return staff.Employee{}, err
	}
	return employee, nil
}

func (s *PostgresStore) SetEmployeeActiveContext(ctx context.Context, id string, active bool) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE employees SET active = $3, updated_at = NOW() WHERE tenant_id = $1 AND id = $2",
		s.tenant, id, active)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) EnsureTenantStaff(ctx context.Context, employees []staff.Employee) error {
	var count int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM employees WHERE tenant_id = $1", s.tenant,
	).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, employee := range employees {
		if _, err := s.SaveEmployeeContext(ctx, employee); err != nil {
			return err
		}
	}
	return nil
}

func scanJobs(rows *sql.Rows) ([]Job, error) {
	jobs := make([]Job, 0)
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.ID, &job.Service, &job.Address, &job.Date, &job.Window,
			&job.Phone, &job.Status, &job.EmployeeID, &job.EmployeeName); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func scanEmployee(row rowScanner) (staff.Employee, error) {
	var employee staff.Employee
	var skills, days, shifts []byte
	err := row.Scan(&employee.ID, &employee.Name, &employee.Role, &employee.Email, &employee.Phone,
		&skills, &days, &shifts, &employee.Active)
	if err != nil {
		return staff.Employee{}, err
	}
	if err := json.Unmarshal(skills, &employee.Skills); err != nil {
		return staff.Employee{}, err
	}
	if err := json.Unmarshal(days, &employee.Days); err != nil {
		return staff.Employee{}, err
	}
	if err := json.Unmarshal(shifts, &employee.Shifts); err != nil {
		return staff.Employee{}, err
	}
	return employee, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	err := row.Scan(&job.ID, &job.Service, &job.Address, &job.Date, &job.Window,
		&job.Phone, &job.Status, &job.EmployeeID, &job.EmployeeName)
	return job, err
}
