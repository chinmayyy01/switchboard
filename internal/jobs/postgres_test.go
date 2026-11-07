package jobs

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/chinmay/switchboard/internal/staff"
)

func TestPostgresBookingIsDurableAndIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenant := newID("test_tenant_")
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM jobs WHERE tenant_id = $1", tenant)
		db.ExecContext(context.Background(), "DELETE FROM employees WHERE tenant_id = $1", tenant)
		db.Close()
	})
	store := NewPostgresStore(db, tenant)
	store.SetWindows([]string{"morning", "afternoon"})
	if _, err := store.SaveEmployeeContext(ctx, staff.Employee{
		ID: "tech-1", Name: "Maya Patel", Role: staff.RoleTechnician,
		Skills: []string{"leak repair"}, Days: []string{"mon"},
		Shifts: []string{"morning"}, Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	request := BookingRequest{
		IdempotencyKey: "provider-call-1/book-1",
		Service:        "leak repair",
		Date:           "2026-11-02",
		Window:         "morning",
		Address:        "1 A Street",
		Phone:          "5551112222",
	}

	const attempts = 8
	results := make(chan Job, attempts)
	errors := make(chan error, attempts)
	var group sync.WaitGroup
	for i := 0; i < attempts; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			job, err := store.BookContext(ctx, request)
			if err != nil {
				errors <- err
				return
			}
			results <- job
		}()
	}
	group.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	var firstID string
	for job := range results {
		if firstID == "" {
			firstID = job.ID
		} else if job.ID != firstID {
			t.Fatalf("idempotent retry returned job %q, want %q", job.ID, firstID)
		}
	}
	if firstID == "" {
		t.Fatal("no booking returned")
	}

	if _, err := store.BookContext(ctx, BookingRequest{
		IdempotencyKey: "provider-call-2/book-1",
		Service:        request.Service,
		Date:           request.Date,
		Window:         request.Window,
		Address:        "2 B Street",
		Phone:          "5553334444",
	}); err != ErrNoStaff {
		t.Fatalf("same employee slot should be reserved once, got %v", err)
	}

	reopened := NewPostgresStore(db, tenant)
	job, found, err := reopened.GetContext(ctx, firstID)
	if err != nil || !found {
		t.Fatalf("persisted job not found: found=%v err=%v", found, err)
	}
	if job.EmployeeID != "tech-1" || job.EmployeeName != "Maya Patel" {
		t.Fatalf("employee assignment not persisted: %+v", job)
	}

	if err := reopened.CancelContext(ctx, firstID); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.BookContext(ctx, BookingRequest{
		IdempotencyKey: "provider-call-3/book-1",
		Service:        request.Service,
		Date:           request.Date,
		Window:         request.Window,
		Address:        "3 C Street",
		Phone:          "5554445555",
	}); err != nil {
		t.Fatalf("cancelled slot should become available: %v", err)
	}

	if err := reopened.SetEmployeeActiveContext(ctx, "tech-1", false); err != nil {
		t.Fatal(err)
	}
	if windows, err := reopened.AvailabilityContext(ctx, "leak repair", request.Date); err != nil || len(windows) != 0 {
		t.Fatalf("inactive technician availability = %v, err=%v", windows, err)
	}
	if err := reopened.SetEmployeeActiveContext(ctx, "tech-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.SaveEmployeeContext(ctx, staff.Employee{
		ID: "tech-1", Name: "Maya Patel", Role: staff.RoleDispatcher,
		Days: []string{"mon"}, Shifts: []string{"morning"}, Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	if windows, err := reopened.AvailabilityContext(ctx, "leak repair", request.Date); err != nil || len(windows) != 0 {
		t.Fatalf("dispatcher should not receive technician visits: %v, err=%v", windows, err)
	}
}

func TestPostgresRegistrySeedsLegacyStaffOnce(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenant := newID("seed_tenant_")
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM jobs WHERE tenant_id = $1", tenant)
		db.ExecContext(context.Background(), "DELETE FROM employees WHERE tenant_id = $1", tenant)
		db.Close()
	})
	staffDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staffDir, tenant), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"employees":[{"id":"tech-1","name":"Maya Patel","skills":["leak repair"],"days":["mon"],"shifts":["morning"]}]}`
	if err := os.WriteFile(filepath.Join(staffDir, tenant, "staff.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := NewPostgresRegistry(ctx, db, staffDir, []string{tenant})
	if err != nil {
		t.Fatal(err)
	}
	employees, err := registry.For(tenant).EmployeesContext(ctx)
	if err != nil || len(employees) != 1 {
		t.Fatalf("seeded employees=%+v err=%v", employees, err)
	}
	if employees[0].Role != staff.RoleTechnician || !employees[0].Active {
		t.Fatalf("legacy employee defaults not applied: %+v", employees[0].Employee)
	}

	employees[0].Role = staff.RoleManager
	if _, err := registry.For(tenant).SaveEmployeeContext(ctx, employees[0].Employee); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPostgresRegistry(ctx, db, staffDir, []string{tenant}); err != nil {
		t.Fatal(err)
	}
	updated, err := registry.For(tenant).EmployeesContext(ctx)
	if err != nil || updated[0].Role != staff.RoleManager {
		t.Fatalf("database changes should not be overwritten by seed files: %+v err=%v", updated, err)
	}
}
