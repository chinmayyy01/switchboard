package agent

import "encoding/json"

type Message struct {
	Role    string
	Content string
}

type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	Name string
	Args json.RawMessage
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
	UserText string
	History  []Message
	Lang     string
}

type Outcome struct {
	Reply      string
	State      string
	ToolCalled *ToolCall
	Citations  []string
	Escalate   bool
	EndCall    bool
	EndReason  string
}
