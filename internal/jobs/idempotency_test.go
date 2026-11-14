package jobs

import (
	"context"
	"testing"

	"github.com/chinmay/switchboard/internal/staff"
)

func TestBookContextReturnsOriginalJobForRepeatedKey(t *testing.T) {
	store := NewStore()
	request := BookingRequest{
		IdempotencyKey: "call-1/book-1",
		Service:        "leak repair",
		Date:           "2026-11-02",
		Window:         "morning",
		Address:        "1 A Street",
		Phone:          "5551112222",
	}

	first, err := store.BookContext(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.BookContext(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("retry created a different job: first=%s second=%s", first.ID, second.ID)
	}
	if got := len(store.ByPhone(request.Phone)); got != 1 {
		t.Fatalf("jobs for phone = %d, want 1", got)
	}
}

func TestOnlyActiveTechniciansAreAssigned(t *testing.T) {
	store := NewStore()
	store.SetStaff(staff.New([]staff.Employee{
		{ID: "dispatch", Name: "Dispatcher", Role: staff.RoleDispatcher, Skills: []string{"leak repair"}, Days: []string{"mon"}, Shifts: []string{"morning"}, Active: true},
		{ID: "tech", Name: "Technician", Role: staff.RoleTechnician, Skills: []string{"leak repair"}, Days: []string{"mon"}, Shifts: []string{"morning"}, Active: true},
	}))

	job, err := store.BookContext(context.Background(), BookingRequest{
		Service: "leak repair", Date: "2026-11-02", Window: "morning",
		Address: "1 A Street", Phone: "5551112222", IdempotencyKey: "call-2/book-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.EmployeeID != "tech" {
		t.Fatalf("assigned employee = %q, want active technician", job.EmployeeID)
	}
}
