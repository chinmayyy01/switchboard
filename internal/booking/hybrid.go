package booking

import (
	"context"
	"strings"

	"github.com/chinmay/switchboard/internal/agent"
)

type Hybrid struct {
	flow agent.Brain
	llm  agent.LLM
}

func NewHybrid(flow agent.Brain, llm agent.LLM) *Hybrid {
	return &Hybrid{flow: flow, llm: llm}
}

func (h *Hybrid) Handle(ctx context.Context, turn agent.Turn) (agent.Outcome, error) {
	out, err := h.flow.Handle(ctx, turn)
	if err != nil || strings.TrimSpace(out.Reply) == "" {
		return out, err
	}

	reply, err := h.llm.Respond(ctx, []agent.Message{
		{
			Role: "system",
			Content: "You are a speech editor for a phone receptionist. Rewrite the line in one or two short, warm, natural spoken sentences. " +
				"Keep the exact same meaning, the same question, and every number, name, and time. " +
				"Do not add facts, apologies, or extra questions. Output only the rewritten line.",
		},
		{Role: "user", Content: out.Reply},
	}, nil)
	if err == nil {
		if text := strings.TrimSpace(reply.Text); text != "" && !strings.Contains(text, "```") {
			out.Reply = text
		}
	}
	return out, nil
}
