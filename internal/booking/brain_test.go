package booking

import (
	"context"
	"testing"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/jobs"
)

func run(t *testing.T, b *Brain, text string) agent.Outcome {
	t.Helper()
	out, err := b.Handle(context.Background(), agent.Turn{UserText: text})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%-28q -> state=%-14s end=%-5v reply=%s", text, out.State, out.EndCall, out.Reply)
	return out
}

func TestBookingFlow(t *testing.T) {
	st := jobs.NewStore()
	b := New(st)

	run(t, b, "my kitchen sink is leaking")
	run(t, b, "12 Main Street")
	run(t, b, "tomorrow morning")
	run(t, b, "5551234567")
	out := run(t, b, "yes")

	if !out.EndCall || out.EndReason != "booked" {
		t.Fatalf("expected booked, got end=%v reason=%s", out.EndCall, out.EndReason)
	}
	found := st.ByPhone("5551234567")
	if len(found) != 1 {
		t.Fatalf("expected 1 job, got %d", len(found))
	}
	if found[0].Service != "leak repair" || found[0].Window != "morning" {
		t.Fatalf("unexpected job: %+v", found[0])
	}
}

func TestEmergencyEscalates(t *testing.T) {
	b := New(jobs.NewStore())
	out := run(t, b, "I smell gas in the kitchen")
	if !out.EndCall || out.EndReason != "escalated" {
		t.Fatalf("expected escalated, got %+v", out)
	}
}

func TestCancelFlow(t *testing.T) {
	st := jobs.NewStore()
	j, err := st.Book("drain cleaning", "2026-10-30", "morning", "1 A Street", "5559998888")
	if err != nil {
		t.Fatal(err)
	}

	b := New(st)
	run(t, b, "I need to cancel my appointment")
	run(t, b, "5559998888")
	out := run(t, b, "yes")

	if !out.EndCall || out.EndReason != "cancelled" {
		t.Fatalf("expected cancelled, got %+v", out)
	}
	got, ok := st.Get(j.ID)
	if !ok || got.Status != "cancelled" {
		t.Fatalf("job not cancelled: %+v", got)
	}
}

func TestRescheduleFlow(t *testing.T) {
	st := jobs.NewStore()
	j, err := st.Book("water heater", "2026-10-30", "morning", "9 B Street", "5551112222")
	if err != nil {
		t.Fatal(err)
	}

	b := New(st)
	run(t, b, "I want to reschedule")
	run(t, b, "5551112222")
	run(t, b, "next tuesday afternoon")
	out := run(t, b, "yes")

	if !out.EndCall || out.EndReason != "rescheduled" {
		t.Fatalf("expected rescheduled, got %+v", out)
	}
	got, _ := st.Get(j.ID)
	if got.Window != "afternoon" {
		t.Fatalf("expected afternoon, got %+v", got)
	}
}
