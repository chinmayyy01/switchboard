package jobs

import (
	"testing"

	"github.com/chinmay/switchboard/internal/staff"
)

func TestBookAssignsStaffAndOffersAlternatives(t *testing.T) {
	st := NewStore()
	st.SetStaff(staff.New([]staff.Employee{
		{ID: "a", Name: "Alex Tech", Skills: []string{"leak repair"}, Shifts: []string{"morning"}},
	}))

	j, err := st.Book("leak repair", "2026-11-02", "morning", "1 A Street", "5551112222")
	if err != nil {
		t.Fatal(err)
	}
	if j.EmployeeName != "Alex Tech" {
		t.Fatalf("employee = %q", j.EmployeeName)
	}

	if _, err := st.Book("leak repair", "2026-11-02", "morning", "2 B Street", "5553334444"); err != ErrNoStaff {
		t.Fatalf("want ErrNoStaff, got %v", err)
	}

	slots := st.Alternatives("leak repair", "2026-11-02", 3)
	if len(slots) == 0 {
		t.Fatal("expected alternatives")
	}
	if slots[0].Date == "2026-11-02" && slots[0].Window == "morning" {
		t.Fatalf("offered the busy slot: %+v", slots[0])
	}

	if err := st.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Book("leak repair", "2026-11-02", "morning", "3 C Street", "5555556666"); err != nil {
		t.Fatalf("slot should be free after cancel: %v", err)
	}
}
