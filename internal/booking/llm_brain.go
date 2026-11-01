package booking

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/jobs"
)

type LLMBrain struct {
	store        *jobs.Store
	kb           Answerer
	llm          agent.LLM
	tenant       string
	businessName string
	services     []string
	history      []agent.Message
	tools        []agent.Tool

	service        string
	servicePending bool
	address        string
	addressPending bool
	date           string
	window         string
	timePending    bool
	phone          string
	phonePending   bool
	detailsPending bool
}

func NewLLM(store *jobs.Store, kb Answerer, llm agent.LLM, tenant, businessName string, services []string) *LLMBrain {
	b := &LLMBrain{
		store:        store,
		kb:           kb,
		llm:          llm,
		tenant:       tenant,
		businessName: businessName,
		services:     services,
	}
	b.tools = b.buildTools()
	return b
}

func (b *LLMBrain) buildTools() []agent.Tool {
	return []agent.Tool{
		{
			Name:        "set_service",
			Description: "Save the service the caller needs.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"service":{"type":"string"}},"required":["service"]}`),
		},
		{
			Name:        "confirm_service",
			Description: "Mark the saved service as confirmed by the caller.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "save_address",
			Description: "Save the service address and get a read-back to confirm with the caller.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"address":{"type":"string"}},"required":["address"]}`),
		},
		{
			Name:        "confirm_address",
			Description: "Mark the saved address as confirmed by the caller.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "set_time",
			Description: "Save the date (ISO YYYY-MM-DD) and window (morning|afternoon|evening) after checking availability.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"date":{"type":"string"},"window":{"type":"string","enum":["morning","afternoon","evening"]}},"required":["date","window"]}`),
		},
		{
			Name:        "confirm_time",
			Description: "Mark the saved day and window as confirmed by the caller.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "check_availability",
			Description: "List open windows for an ISO date.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"date":{"type":"string"}},"required":["date"]}`),
		},
		{
			Name:        "set_phone",
			Description: "Save the caller's 10-digit callback number and get a spoken read-back.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"phone":{"type":"string"}},"required":["phone"]}`),
		},
		{
			Name:        "confirm_phone",
			Description: "Mark the saved callback number as confirmed by the caller.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "confirm_details",
			Description: "Read back the full booking details for a final confirmation.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "book",
			Description: "Book the visit using the saved details. Only call after the caller confirms.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "lookup_job",
			Description: "Find existing bookings by the caller's phone number.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"phone":{"type":"string"}},"required":["phone"]}`),
		},
		{
			Name:        "reschedule",
			Description: "Move the caller's booking to a new date and window.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"phone":{"type":"string"},"date":{"type":"string"},"window":{"type":"string","enum":["morning","afternoon","evening"]}},"required":["phone","date","window"]}`),
		},
		{
			Name:        "cancel",
			Description: "Cancel the caller's booking.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"phone":{"type":"string"}},"required":["phone"]}`),
		},
		{
			Name:        "lookup_policy",
			Description: "Answer a policy or FAQ question from the knowledge base.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		},
		{
			Name:        "transfer_to_human",
			Description: "Warm-transfer the caller to a human.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"reason":{"type":"string"}},"required":["reason"]}`),
		},
	}
}

func (b *LLMBrain) nextSlot() string {
	switch {
	case b.service == "":
		return "which service they need (leak repair, drain cleaning, water heater, AC, heating)"
	case b.servicePending:
		return "confirmation that the service is correct"
	case b.address == "":
		return "the service address"
	case b.addressPending:
		return "confirmation that the address is correct"
	case b.date == "" || b.window == "":
		return "which day and window works (morning, afternoon, evening)"
	case b.timePending:
		return "confirmation that the day and window are correct"
	case b.phone == "":
		return "a 10-digit callback number"
	case b.phonePending:
		return "confirmation that the callback number is correct"
	case b.detailsPending:
		return "confirmation to book the visit"
	default:
		return "confirmation of the full booking details"
	}
}

func (b *LLMBrain) activeTools(userText string) []agent.Tool {
	names := map[string]bool{
		"lookup_policy":     true,
		"transfer_to_human": true,
	}
	low := strings.ToLower(userText)
	if hasAny(low, []string{"reschedule", "cancel", "move my", "existing", "my booking", "appointment"}) {
		names["lookup_job"] = true
		names["reschedule"] = true
		names["cancel"] = true
	}
	switch {
	case b.service == "":
		names["set_service"] = true
	case b.servicePending:
		names["confirm_service"] = true
		names["set_service"] = true
	case b.address == "":
		names["save_address"] = true
	case b.addressPending:
		names["confirm_address"] = true
		names["save_address"] = true
	case b.date == "" || b.window == "":
		names["check_availability"] = true
		names["set_time"] = true
	case b.timePending:
		names["confirm_time"] = true
		names["set_time"] = true
	case b.phone == "":
		names["set_phone"] = true
	case b.phonePending:
		names["confirm_phone"] = true
		names["set_phone"] = true
	case b.detailsPending:
		names["book"] = true
	default:
		names["confirm_details"] = true
	}

	out := make([]agent.Tool, 0, len(names))
	for _, t := range b.tools {
		if names[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

func (b *LLMBrain) systemPrompt(lang string) string {
	language := "English"
	if strings.HasPrefix(lang, "hi") {
		language = "Hindi"
	}
	today := time.Now().Format("2006-01-02")
	state := fmt.Sprintf(
		`service=%q (confirmed=%v), address=%q (confirmed=%v), date=%q, window=%q (confirmed=%v), phone=%q (confirmed=%v)`,
		b.service, !b.servicePending && b.service != "",
		b.address, !b.addressPending && b.address != "",
		b.date, b.window, !b.timePending && b.date != "",
		b.phone, !b.phonePending && b.phone != "",
	)

	var next string
	switch {
	case b.service == "":
		next = "The next required item is the service. If the caller's message describes a problem, call set_service immediately with the closest category (leak repair, drain cleaning, water heater, AC, heating) and say nothing else. Only ask what they need if no service is clear."
	case b.servicePending:
		next = fmt.Sprintf("The service %q is saved but not confirmed. If the caller's latest message agrees, call confirm_service immediately. Otherwise say a short read-back and ask if %q is right. Do not move on until it is confirmed.", b.service, b.service)
	case b.address == "":
		next = "The next required item is the service address. If the caller's message contains an address, call save_address immediately and say nothing else. Only ask for the address if it is not clear."
	case b.addressPending:
		next = fmt.Sprintf("The address %q is saved but not confirmed. If the caller's latest message agrees, call confirm_address immediately. Otherwise read back %q and ask if it is right.", b.address, b.address)
	case b.date == "" || b.window == "":
		next = fmt.Sprintf("The next required item is the day and window. Today is %s. If the caller gives a day or time, call set_time immediately with an ISO date and morning/afternoon/evening; use check_availability if needed. Only ask if no time is clear.", today)
	case b.timePending:
		next = fmt.Sprintf("The day %q and window %q are saved but not confirmed. If the caller's latest message agrees, call confirm_time immediately. Otherwise read back %s %s and ask if it works.", b.date, b.window, b.date, b.window)
	case b.phone == "":
		next = "The next required item is a 10-digit callback number. If the caller's message contains digits, call set_phone immediately and say nothing else. Only ask if no number is clear."
	case b.phonePending:
		next = fmt.Sprintf("The callback number %q is saved but not confirmed. If the caller's latest message agrees, call confirm_phone immediately. Otherwise read back the number and ask if it is right.", b.phone)
	case b.detailsPending:
		next = "The full booking was just read back. If the caller's latest message agrees, call book immediately. If they want a change, ask what to change."
	default:
		next = "All details are saved and confirmed. Call confirm_details now, then read back the booking and ask for a yes. Only call book after the caller confirms."
	}

	return fmt.Sprintf(`You are Alex, a warm receptionist for %s. You are on a live call. Speak briefly and naturally in %s.
Booking so far: %s.
%s
Sound like a person on the phone: short sentences, contractions, and occasional natural acknowledgments ("Okay.", "Got it.", "Sure.", "Mm-hmm."). Do not overuse fillers or sound like a form. Use the full conversation above; never ask for a detail the caller already gave.
Say dates naturally ("November 2nd" or "Monday"), never ISO dates. Never read internal job IDs or system references to the caller.
Never claim you saved, recorded, or booked something unless a tool returned ok.
For gas, flooding, no heat, electrical hazards, or sewage, call transfer_to_human. For hours, pricing, warranty, or service area, call lookup_policy and answer only from its result.
Current date: %s. Never reveal reasoning, plans, tool names, or JSON.`,
		b.businessName, language, state, next, today)
}

func (b *LLMBrain) Handle(ctx context.Context, turn agent.Turn) (agent.Outcome, error) {
	userText := strings.TrimSpace(turn.UserText)
	low := strings.ToLower(userText)

	if hasAny(low, emergencyWords) {
		return b.escalate("This sounds like an emergency. I'm connecting you to a person now.", "emergency"), nil
	}
	if hasAny(low, humanWords) {
		return b.escalate("Of course, connecting you to a person.", "caller_requested_human"), nil
	}

	b.history = append(b.history, agent.Message{Role: "user", Content: userText})
	msgs := make([]agent.Message, 0, len(b.history)+1)
	msgs = append(msgs, agent.Message{Role: "system", Content: b.systemPrompt(turn.Lang)})
	msgs = append(msgs, b.history...)

	var citations []string
	var pendingEnd string
	var lastTool *agent.ToolCall
	nudged := false

	streamer, canStream := b.llm.(agent.Streamer)
	streaming := canStream && turn.OnSentence != nil
	streamed := false
	var sentenceBuf strings.Builder

	for i := 0; i < 8; i++ {
		useStream := streaming && i > 0
		tools := b.activeTools(low)

		var reply agent.Reply
		var err error
		if useStream {
			reply, err = streamer.Stream(ctx, msgs, tools, func(token string) error {
				sentenceBuf.WriteString(token)
				return flushSentences(&sentenceBuf, turn.OnSentence, &streamed)
			})
		} else {
			reply, err = b.llm.Respond(ctx, msgs, tools)
		}
		if err != nil {
			if useStream && streamed {
				return agent.Outcome{}, err
			}
			slog.Error("llm", "err", err, "stream", useStream)
			return b.escalate("I'm having trouble hearing you. Let me connect you to a person.", "llm_error"), nil
		}

		if reply.ToolCall == nil {
			if useStream {
				if err := flushSentences(&sentenceBuf, turn.OnSentence, &streamed); err != nil {
					return agent.Outcome{}, err
				}
				if tail := strings.TrimSpace(sentenceBuf.String()); tail != "" {
					if err := turn.OnSentence(tail); err != nil {
						return agent.Outcome{}, err
					}
					streamed = true
					sentenceBuf.Reset()
				}
			}

			text := strings.TrimSpace(reply.Text)
			if looksLikeToolNarration(text) {
				if nudged {
					return b.escalate("Sorry, I'm having trouble. Let me connect you to a person.", "tool_narration"), nil
				}
				nudged = true
				msgs = append(msgs, agent.Message{
					Role:    "system",
					Content: "Do not describe tool calls or your reasoning. Either call the tool with the correct arguments, or reply to the caller in one or two sentences.",
				})
				continue
			}
			if text == "" {
				text = "Sorry, could you say that again?"
			}
			b.history = append(b.history, agent.Message{Role: "assistant", Content: text})
			out := agent.Outcome{
				Reply:      text,
				State:      "conversation",
				ToolCalled: lastTool,
				Citations:  citations,
				Streamed:   streamed,
			}
			if pendingEnd != "" {
				out.EndCall = true
				out.EndReason = pendingEnd
			}
			return out, nil
		}

		sentenceBuf.Reset()
		call := *reply.ToolCall
		lastTool = &call

		result, cites, end, terminal := b.execTool(ctx, call, turn.Lang)
		if terminal != nil {
			b.history = append(b.history, agent.Message{Role: "assistant", Content: terminal.Reply})
			return *terminal, nil
		}
		if len(cites) > 0 {
			citations = cites
		}
		if end != "" {
			pendingEnd = end
		}

		msgs = append(msgs, agent.Message{Role: "assistant", ToolCalls: []agent.ToolCall{call}})
		msgs = append(msgs, agent.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
		b.history = append(b.history, agent.Message{Role: "assistant", ToolCalls: []agent.ToolCall{call}})
		b.history = append(b.history, agent.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
		msgs[0] = agent.Message{Role: "system", Content: b.systemPrompt(turn.Lang)}
	}

	return b.escalate("Let me connect you to a person who can help.", "loop_limit"), nil
}

func (b *LLMBrain) execTool(ctx context.Context, call agent.ToolCall, lang string) (string, []string, string, *agent.Outcome) {
	switch call.Name {
	case "set_service":
		var a struct {
			Service string `json:"service"`
		}
		json.Unmarshal(call.Args, &a)
		if strings.TrimSpace(a.Service) == "" {
			return jsonResult(map[string]any{"error": "service required"}), nil, "", nil
		}
		b.service = a.Service
		b.servicePending = true
		b.detailsPending = false
		return jsonResult(map[string]any{
			"ok":        true,
			"service":   b.service,
			"read_back": "Got it, " + b.service + ". Is that right?",
		}), nil, "", nil

	case "confirm_service":
		if b.service == "" {
			return jsonResult(map[string]any{"ok": false, "error": "no service saved"}), nil, "", nil
		}
		b.servicePending = false
		return jsonResult(map[string]any{"ok": true, "service": b.service}), nil, "", nil

	case "save_address":
		var a struct {
			Address string `json:"address"`
		}
		json.Unmarshal(call.Args, &a)
		addr := cleanAddress(a.Address)
		if !validAddress(addr) {
			return jsonResult(map[string]any{"ok": false, "error": "I didn't catch that address"}), nil, "", nil
		}
		b.address = addr
		b.addressPending = true
		b.detailsPending = false
		return jsonResult(map[string]any{
			"ok":        true,
			"address":   addr,
			"read_back": "Just to confirm, the service address is " + addr + " — is that right?",
		}), nil, "", nil

	case "confirm_address":
		if b.address == "" {
			return jsonResult(map[string]any{"ok": false, "error": "no address saved"}), nil, "", nil
		}
		b.addressPending = false
		return jsonResult(map[string]any{"ok": true, "address": b.address}), nil, "", nil

	case "set_time":
		var a struct {
			Date   string `json:"date"`
			Window string `json:"window"`
		}
		json.Unmarshal(call.Args, &a)
		if a.Date == "" || !jobs.ValidWindow(a.Window) {
			return jsonResult(map[string]any{"ok": false, "error": "date and window required"}), nil, "", nil
		}
		avail := b.store.Availability(a.Date)
		if !contains(avail, a.Window) {
			return jsonResult(map[string]any{"ok": false, "windows": avail}), nil, "", nil
		}
		b.date = a.Date
		b.window = a.Window
		b.timePending = true
		b.detailsPending = false
		return jsonResult(map[string]any{
			"ok":        true,
			"date":      b.date,
			"window":    b.window,
			"read_back": "Just to confirm, " + humanDate(b.date) + " in the " + b.window + ". Does that work?",
		}), nil, "", nil

	case "confirm_time":
		if b.date == "" || b.window == "" {
			return jsonResult(map[string]any{"ok": false, "error": "no time saved"}), nil, "", nil
		}
		b.timePending = false
		return jsonResult(map[string]any{"ok": true, "date": b.date, "window": b.window}), nil, "", nil

	case "check_availability":
		var a struct {
			Date string `json:"date"`
		}
		json.Unmarshal(call.Args, &a)
		return jsonResult(map[string]any{"date": a.Date, "windows": b.store.Availability(a.Date)}), nil, "", nil

	case "set_phone":
		var a struct {
			Phone string `json:"phone"`
		}
		json.Unmarshal(call.Args, &a)
		phone := validPhone(digitsFromSpeech(a.Phone))
		if phone == "" {
			return jsonResult(map[string]any{"ok": false, "error": "need 10 digits"}), nil, "", nil
		}
		b.phone = phone
		b.phonePending = true
		b.detailsPending = false
		return jsonResult(map[string]any{
			"ok":        true,
			"phone":     phone,
			"read_back": "I have " + speakPhone(phone) + ".",
		}), nil, "", nil

	case "confirm_phone":
		if b.phone == "" {
			return jsonResult(map[string]any{"ok": false, "error": "no phone saved"}), nil, "", nil
		}
		b.phonePending = false
		return jsonResult(map[string]any{"ok": true, "phone": b.phone}), nil, "", nil

	case "confirm_details":
		if b.service == "" || b.address == "" || b.date == "" || b.window == "" || b.phone == "" {
			return jsonResult(map[string]any{"ok": false, "error": "missing " + b.nextSlot()}), nil, "", nil
		}
		b.detailsPending = true
		return jsonResult(map[string]any{
			"ok": true,
			"read_back": fmt.Sprintf("%s at %s on %s, %s window, callback %s. Shall I book it?",
				b.service, b.address, humanDate(b.date), b.window, speakPhone(b.phone)),
		}), nil, "", nil

	case "book":
		if b.service == "" || b.address == "" || b.date == "" || b.window == "" || b.phone == "" ||
			b.servicePending || b.addressPending || b.timePending || b.phonePending || !b.detailsPending {
			return jsonResult(map[string]any{"ok": false, "error": "read back the details first"}), nil, "", nil
		}
		j, err := b.store.Book(b.service, b.date, b.window, b.address, b.phone)
		if err != nil {
			return jsonResult(map[string]any{"ok": false, "error": err.Error()}), nil, "", nil
		}
		return jsonResult(map[string]any{
			"ok": true, "job_id": j.ID, "service": j.Service, "date": j.Date,
			"window": j.Window, "address": j.Address, "phone_spoken": speakPhone(j.Phone),
		}), nil, "booked", nil

	case "lookup_job":
		var a struct {
			Phone string `json:"phone"`
		}
		json.Unmarshal(call.Args, &a)
		found := b.store.ByPhone(validPhone(digitsFromSpeech(a.Phone)))
		list := make([]map[string]any, 0, len(found))
		for _, j := range found {
			list = append(list, map[string]any{
				"job_id": j.ID, "service": j.Service, "date": j.Date,
				"window": j.Window, "address": j.Address,
			})
		}
		return jsonResult(map[string]any{"jobs": list}), nil, "", nil

	case "reschedule":
		var a struct {
			Phone  string `json:"phone"`
			Date   string `json:"date"`
			Window string `json:"window"`
		}
		json.Unmarshal(call.Args, &a)
		found := b.store.ByPhone(validPhone(digitsFromSpeech(a.Phone)))
		if len(found) == 0 {
			return jsonResult(map[string]any{"ok": false, "error": "no booking found"}), nil, "", nil
		}
		if err := b.store.Reschedule(found[0].ID, a.Date, a.Window); err != nil {
			return jsonResult(map[string]any{"ok": false, "error": err.Error()}), nil, "", nil
		}
		return jsonResult(map[string]any{"ok": true, "job_id": found[0].ID, "date": a.Date, "window": a.Window}), nil, "rescheduled", nil

	case "cancel":
		var a struct {
			Phone string `json:"phone"`
		}
		json.Unmarshal(call.Args, &a)
		found := b.store.ByPhone(validPhone(digitsFromSpeech(a.Phone)))
		if len(found) == 0 {
			return jsonResult(map[string]any{"ok": false, "error": "no booking found"}), nil, "", nil
		}
		if err := b.store.Cancel(found[0].ID); err != nil {
			return jsonResult(map[string]any{"ok": false, "error": err.Error()}), nil, "", nil
		}
		return jsonResult(map[string]any{"ok": true, "job_id": found[0].ID}), nil, "cancelled", nil

	case "lookup_policy":
		var a struct {
			Query string `json:"query"`
		}
		json.Unmarshal(call.Args, &a)
		if b.kb == nil {
			return jsonResult(map[string]any{"grounded": false}), nil, "", b.escalatePtr("I'm not sure about that. Let me connect you to a person.", "ungrounded")
		}
		res, err := b.kb.Answer(ctx, b.tenant, a.Query, 3, lang)
		if err != nil || !res.Grounded {
			return jsonResult(map[string]any{"grounded": false}), nil, "", b.escalatePtr("I'm not sure about that. Let me connect you to a person.", "ungrounded")
		}
		return jsonResult(map[string]any{"grounded": true, "answer": res.Answer, "citations": res.Citations}), res.Citations, "", nil

	case "transfer_to_human":
		var a struct {
			Reason string `json:"reason"`
		}
		json.Unmarshal(call.Args, &a)
		reason := a.Reason
		if reason == "" {
			reason = "caller_requested_human"
		}
		return "", nil, "", b.escalatePtr("Of course, connecting you to a person now.", reason)
	}

	return jsonResult(map[string]any{"error": "unknown tool"}), nil, "", nil
}

func (b *LLMBrain) escalate(reply, reason string) agent.Outcome {
	return agent.Outcome{Reply: reply, State: "escalate", Escalate: true, EndCall: true, EndReason: reason}
}

func (b *LLMBrain) escalatePtr(reply, reason string) *agent.Outcome {
	out := b.escalate(reply, reason)
	return &out
}

var narrationMarkers = []string{
	"check_availability", "lookup_job", "lookup_policy", "transfer_to_human",
	"wait for response", "call the tool", "tool_call", "proceed to book",
	"arguments:", "tool call", "i will call", "i'll call", "step 1",
	"the caller wants", "the user wants", "my plan", "first, i",
	"i need to call", "let me call the",
}

func looksLikeToolNarration(text string) bool {
	low := strings.ToLower(text)
	for _, marker := range narrationMarkers {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func jsonResult(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func humanDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("January 2")
}

func sentenceEnd(s string) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.', '!', '?', '\n':
			return i
		}
	}
	return -1
}

func flushSentences(buf *strings.Builder, emit func(string) error, streamed *bool) error {
	for {
		s := buf.String()
		idx := sentenceEnd(s)
		if idx < 0 {
			return nil
		}
		part := strings.TrimSpace(s[:idx+1])
		buf.Reset()
		buf.WriteString(s[idx+1:])
		if part == "" {
			continue
		}
		if err := emit(part); err != nil {
			return err
		}
		*streamed = true
	}
}
