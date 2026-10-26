package booking

import (
	"context"
	"strings"
	"testing"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/ai"
	"github.com/chinmay/switchboard/internal/jobs"
)

type stubLLM struct{ reply string }

func (s stubLLM) Respond(_ context.Context, _ []agent.Message, _ []agent.Tool) (agent.Reply, error) {
	return agent.Reply{Text: s.reply}, nil
}

type stubSelector struct{ llm agent.LLM }

func (s stubSelector) Select(string) agent.Providers {
	return agent.Providers{STT: agent.NewFakeSTT(), TTS: agent.NewFakeTTS(), LLM: s.llm}
}

type stubKB struct{ res ai.RetrieveResponse }

func (s stubKB) Retrieve(_ context.Context, _, _ string, _ int, _ []string) (ai.RetrieveResponse, error) {
	return s.res, nil
}

func newBrain(st *jobs.Store) *Brain {
	return New(st, agent.NewFakeSelector(), nil, "rivera")
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
	kb := stubKB{res: ai.RetrieveResponse{
		Grounded: true,
		TopScore: 0.8,
		Results: []ai.Result{{
			DocID: "faq/hours",
			Title: "hours",
			Type:  "faq",
			Text:  "We are open Monday to Friday from eight to six.",
			Score: 0.8,
		}},
	}}
	sel := stubSelector{llm: stubLLM{reply: "We're open Monday to Friday, 8am to 6pm."}}
	b := New(jobs.NewStore(), sel, kb, "rivera")

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
	kb := stubKB{res: ai.RetrieveResponse{Grounded: false, TopScore: 0.2}}
	sel := stubSelector{llm: stubLLM{reply: "should not be used"}}
	b := New(jobs.NewStore(), sel, kb, "rivera")

	out := run(t, b, "do you offer a senior discount?")
	if !out.Escalate || out.EndReason != "ungrounded" {
		t.Fatalf("expected ungrounded escalation, got %+v", out)
	}
}

func TestFAQWithoutKnowledgeEscalates(t *testing.T) {
	b := New(jobs.NewStore(), agent.NewFakeSelector(), nil, "rivera")
	out := run(t, b, "what is your warranty?")
	if !out.Escalate || out.EndReason != "ungrounded" {
		t.Fatalf("expected ungrounded escalation, got %+v", out)
	}
}
