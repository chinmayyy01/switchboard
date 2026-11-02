package booking

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/staff"
)

type scriptedLLM struct {
	replies []agent.Reply
	i       int
}

func (s *scriptedLLM) Respond(_ context.Context, _ []agent.Message, _ []agent.Tool) (agent.Reply, error) {
	r := s.replies[s.i]
	if s.i < len(s.replies)-1 {
		s.i++
	}
	return r, nil
}

func TestLLMBrainBooksAfterToolCall(t *testing.T) {
	st := jobs.NewStore()
	llm := &scriptedLLM{replies: []agent.Reply{
		{ToolCall: &agent.ToolCall{ID: "call_1", Name: "book"}},
		{Text: "You're booked for Friday morning. Anything else?"},
	}}
	b := NewLLM(st, nil, llm, "rivera", "Rivera Plumbing", []string{"leak repair"})
	b.service = "leak repair"
	b.address = "12 Main St"
	b.date = "2026-10-30"
	b.window = "morning"
	b.phone = "6204162579"
	b.detailsPending = true

	out, err := b.Handle(context.Background(), agent.Turn{UserText: "yes book it", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.EndCall || out.EndReason != "booked" {
		t.Fatalf("unexpected outcome: %+v", out)
	}
	if len(st.ByPhone("6204162579")) != 1 {
		t.Fatal("job was not booked")
	}
	t.Logf("reply=%s", out.Reply)
}

func TestLLMBrainAddressConfirmation(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"address": "37 Pragati Apartments"})
	llm := &scriptedLLM{replies: []agent.Reply{
		{ToolCall: &agent.ToolCall{ID: "call_1", Name: "save_address", Args: args}},
		{Text: "Just to confirm, the service address is 37 Pragati Apartments — is that right?"},
	}}
	b := NewLLM(jobs.NewStore(), nil, llm, "rivera", "Rivera Plumbing", nil)

	out, err := b.Handle(context.Background(), agent.Turn{UserText: "37 Pragati Apartments", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if b.address != "37 Pragati Apartments" {
		t.Fatalf("address not saved: %q", b.address)
	}
	if !b.addressPending {
		t.Fatal("address should await confirmation")
	}
	if !strings.Contains(out.Reply, "confirm") {
		t.Fatalf("expected read-back, got %q", out.Reply)
	}
}

func TestLLMBrainTransfers(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"reason": "emergency"})
	llm := &scriptedLLM{replies: []agent.Reply{
		{ToolCall: &agent.ToolCall{ID: "call_1", Name: "transfer_to_human", Args: args}},
	}}
	b := NewLLM(jobs.NewStore(), nil, llm, "rivera", "Rivera Plumbing", nil)

	out, err := b.Handle(context.Background(), agent.Turn{UserText: "I need help with something", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Escalate || out.EndReason != "emergency" {
		t.Fatalf("unexpected outcome: %+v", out)
	}
}

func TestLLMBrainRetriesToolNarration(t *testing.T) {
	llm := &scriptedLLM{replies: []agent.Reply{
		{Text: "Call check_availability with date 2026-12-13 and wait for response."},
		{Text: "Let me check that for you."},
	}}
	b := NewLLM(jobs.NewStore(), nil, llm, "rivera", "Rivera Plumbing", nil)

	out, err := b.Handle(context.Background(), agent.Turn{UserText: "is the 13th free?", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.Reply, "check_availability") {
		t.Fatalf("tool narration was spoken: %q", out.Reply)
	}
	if out.Escalate {
		t.Fatalf("should retry before escalating: %+v", out)
	}
}

func TestLLMBrainEscalatesOnPersistentNarration(t *testing.T) {
	llm := &scriptedLLM{replies: []agent.Reply{
		{Text: "Call check_availability."},
		{Text: "Call check_availability again."},
	}}
	b := NewLLM(jobs.NewStore(), nil, llm, "rivera", "Rivera Plumbing", nil)

	out, err := b.Handle(context.Background(), agent.Turn{UserText: "check the 13th", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Escalate || out.EndReason != "tool_narration" {
		t.Fatalf("expected tool_narration escalation: %+v", out)
	}
}

func TestSetTimeOffersAlternativesWhenStaffBusy(t *testing.T) {
	st := jobs.NewStore()
	st.SetStaff(staff.New([]staff.Employee{
		{ID: "a", Name: "A", Skills: []string{"heating"}, Shifts: []string{"morning"}},
	}))
	if _, err := st.Book("heating", "2026-11-02", "morning", "1 A Street", "5551112222"); err != nil {
		t.Fatal(err)
	}
	b := NewLLM(st, nil, nil, "rivera", "Rivera Plumbing", nil)
	b.service = "heating"
	args, _ := json.Marshal(map[string]string{"date": "2026-11-02", "window": "morning"})
	result, _, _, _ := b.execTool(context.Background(), agent.ToolCall{Name: "set_time", Args: args}, "en")
	if !strings.Contains(result, "alternatives") {
		t.Fatalf("missing alternatives: %s", result)
	}
	if strings.Contains(result, `"ok":true`) {
		t.Fatalf("should not accept a busy slot: %s", result)
	}
}
