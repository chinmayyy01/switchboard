package staff

import "testing"

func TestAvailabilityAndAssign(t *testing.T) {
	r := New([]Employee{
		{ID: "a", Name: "A", Skills: []string{"leak repair"}, Days: []string{"mon"}, Shifts: []string{"morning"}},
		{ID: "b", Name: "B", Skills: []string{"leak repair"}, Days: []string{"mon"}, Shifts: []string{"morning", "afternoon"}},
	})
	date := "2026-11-02" // Monday

	if got := len(r.Available("leak repair", date, "morning")); got != 2 {
		t.Fatalf("morning: want 2, got %d", got)
	}
	if got := len(r.Available("leak repair", date, "afternoon")); got != 1 {
		t.Fatalf("afternoon: want 1, got %d", got)
	}
	if got := len(r.Available("drain cleaning", date, "morning")); got != 0 {
		t.Fatalf("wrong skill: want 0, got %d", got)
	}

	e, ok := r.Assign("leak repair", date, "morning")
	if !ok {
		t.Fatal("assign failed")
	}
	if got := len(r.Available("leak repair", date, "morning")); got != 1 {
		t.Fatalf("after assign: want 1, got %d", got)
	}
	r.Release(e.ID, date, "morning")
	if got := len(r.Available("leak repair", date, "morning")); got != 2 {
		t.Fatalf("after release: want 2, got %d", got)
	}
}
