package agent

import "encoding/json"

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolName   string     `json:"tool_name,omitempty"`
}

type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type Reply struct {
	Text     string
	ToolCall *ToolCall
}

type Transcript struct {
	Text       string
	Lang       string
	Confidence float64
}

type Turn struct {
	UserText   string
	History    []Message
	Lang       string
	OnSentence func(string) error
}

type Outcome struct {
	Reply      string
	State      string
	ToolCalled *ToolCall
	Citations  []string
	Escalate   bool
	EndCall    bool
	EndReason  string
	Streamed   bool
}
