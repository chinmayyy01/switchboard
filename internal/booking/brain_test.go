package booking

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/ai"
	"github.com/chinmay/switchboard/internal/jobs"
)

type stubKB struct{ res ai.AnswerResponse }

func (s stubKB) Answer(_ context.Context, _, _ string, _ int, _ string) (ai.AnswerResponse, error) {
	return s.res, nil
}

func newBrain(st *jobs.Store) *Brain {
	return New(st, nil, "rivera", "Rivera Plumbing")
}

func run(t *testing.T, b *Brain, text string) agent.Outcome {
	t.Helper()
	out, err := b.Handle(context.Background(), agent.Turn{UserText: text})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%-34q -> state=%-12s end=%-5v escalate=%-5v reply=%s", text, out.State, out.EndCall, out.Escalate, out.Reply)
	return out
}

func TestBookingFlow(t *testing.T) {
	st := jobs.NewStore()
	b := newBrain(st)

	run(t, b, "my kitchen sink is leaking")
	run(t, b, "12 Main Street")
	run(t, b, "tomorrow morning")
	run(t, b, "5551234567")
	out := run(t, b, "yes")

	if !out.EndCall || out.EndReason != "booked" {
		t.Fatalf("expected booked, got end=%v reason=%s", out.EndCall, out.EndReason)
	}
	found := st.ByPhone("5551234567")
	if len(found) != 1 || found[0].Service != "leak repair" || found[0].Window != "morning" {
		t.Fatalf("unexpected job: %+v", found)
	}
}

func TestEmergencyEscalates(t *testing.T) {
	b := newBrain(jobs.NewStore())
	out := run(t, b, "I smell gas in the kitchen")
	if !out.Escalate || !out.EndCall || out.EndReason != "emergency" {
		t.Fatalf("expected emergency escalation, got %+v", out)
	}
}

func TestCancelFlow(t *testing.T) {
	st := jobs.NewStore()
	j, err := st.Book("drain cleaning", "2026-10-30", "morning", "1 A Street", "5559998888")
	if err != nil {
		t.Fatal(err)
	}
	b := newBrain(st)
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
	b := newBrain(st)
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

func TestFAQGroundsAndCites(t *testing.T) {
	kb := stubKB{res: ai.AnswerResponse{
		Grounded:  true,
		Answer:    "We're open Monday to Friday, 8am to 6pm.",
		Citations: []string{"hours (faq/hours)"},
	}}
	b := New(jobs.NewStore(), kb, "rivera", "Rivera Plumbing")

	out := run(t, b, "what are your hours?")
	if out.State != "faq" || out.Escalate {
		t.Fatalf("expected grounded faq, got %+v", out)
	}
	if !strings.Contains(out.Reply, "Monday") {
		t.Fatalf("unexpected reply: %s", out.Reply)
	}
	if len(out.Citations) == 0 || !strings.Contains(out.Citations[0], "faq/hours") {
		t.Fatalf("missing citation: %+v", out.Citations)
	}
}

func TestFAQAbstainsAndEscalates(t *testing.T) {
	kb := stubKB{res: ai.AnswerResponse{Grounded: false}}
	b := New(jobs.NewStore(), kb, "rivera", "Rivera Plumbing")

	out := run(t, b, "do you offer a senior discount?")
	if !out.Escalate || out.EndReason != "ungrounded" {
		t.Fatalf("expected ungrounded escalation, got %+v", out)
	}
}

func TestFAQWithoutAnswererEscalates(t *testing.T) {
	b := New(jobs.NewStore(), nil, "rivera", "Rivera Plumbing")
	out := run(t, b, "what is your warranty?")
	if !out.Escalate || out.EndReason != "ungrounded" {
		t.Fatalf("expected ungrounded escalation, got %+v", out)
	}
}

func TestPhoneParsing(t *testing.T) {
	cases := map[string]string{
		"620 416 2579":                                    "6204162579",
		"(620) 416-2579":                                  "6204162579",
		"six two zero, four one six, two five seven nine": "6204162579",
		"620 to 4162579":                                  "6204162579",
	}
	for in, want := range cases {
		if got := digitsFromSpeech(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
	if validPhone("620416257") != "" {
		t.Error("9-digit number should be invalid")
	}
	if validPhone("16204162579") != "6204162579" {
		t.Error("leading country code not stripped")
	}
	if speakPhone("6204162579") != "6 2 0 4 1 6 2 5 7 9" {
		t.Errorf("unexpected spoken form: %q", speakPhone("6204162579"))
	}
}

func TestDateParsing(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.Local)
	cases := map[string]string{
		"tomorrow":     "2026-10-29",
		"next tuesday": "2026-11-03",
		"november 5":   "2026-11-05",
		"the 5th":      "2026-11-05",
		"in 3 days":    "2026-10-31",
		"2026-12-01":   "2026-12-01",
	}
	for in, want := range cases {
		if got := parseDate(in, now); got != want {
			t.Errorf("%q: got %s want %s", in, got, want)
		}
	}
}

func TestClosingAfterBooking(t *testing.T) {
	st := jobs.NewStore()
	b := New(st, nil, "rivera", "Rivera Plumbing")
	for _, s := range []string{"my kitchen sink is leaking", "12 Main Street", "tomorrow morning", "6204162579", "yes"} {
		run(t, b, s)
	}
	out := run(t, b, "thank you")
	if !out.EndCall || out.EndReason != "hangup" || out.Escalate {
		t.Fatalf("expected polite hangup, got %+v", out)
	}
}

func TestAddressRejectsFiller(t *testing.T) {
	b := newBrain(jobs.NewStore())
	run(t, b, "my ac is broken")
	out := run(t, b, "my address is um")
	if out.State != "address" {
		t.Fatalf("expected to re-ask address, got %+v", out)
	}
	out = run(t, b, "37 Pragati Apartments")
	if out.State != "time" {
		t.Fatalf("expected time, got %+v", out)
	}
}
