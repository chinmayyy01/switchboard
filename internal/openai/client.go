package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/agent"
)

type Client struct {
	APIKey          string
	BaseURL         string
	Model           string
	ReasoningEffort string
	HTTP            *http.Client
}

func NewFromEnv() *Client {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil
	}
	base := os.Getenv("OPENAI_BASE_URL")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Client{
		APIKey:  key,
		BaseURL: base,
		Model:   model,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

func NewSarvamFromEnv() *Client {
	key := os.Getenv("SARVAM_API_KEY")
	if key == "" {
		return nil
	}
	model := os.Getenv("SARVAM_MODEL")
	if model == "" {
		model = "sarvam-105b-conversations"
	}
	return &Client{
		APIKey:          key,
		BaseURL:         "https://api.sarvam.ai/v1",
		Model:           model,
		ReasoningEffort: "low",
		HTTP:            &http.Client{Timeout: 60 * time.Second},
	}
}

type apiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type apiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function apiFunction `json:"function"`
}

type apiMessage struct {
	Role       string        `json:"role"`
	Content    any           `json:"content"`
	ToolCalls  []apiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type apiTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

func (c *Client) payload(convo []agent.Message, tools []agent.Tool) map[string]any {
	msgs := make([]apiMessage, 0, len(convo))
	for _, m := range convo {
		am := apiMessage{Role: m.Role, Content: m.Content}
		if len(m.ToolCalls) > 0 {
			am.Content = nil
			for _, tc := range m.ToolCalls {
				am.ToolCalls = append(am.ToolCalls, apiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: apiFunction{
						Name:      tc.Name,
						Arguments: string(tc.Args),
					},
				})
			}
		}
		if m.Role == "tool" {
			am.ToolCallID = m.ToolCallID
		}
		msgs = append(msgs, am)
	}

	apiTools := make([]apiTool, 0, len(tools))
	for _, t := range tools {
		var at apiTool
		at.Type = "function"
		at.Function.Name = t.Name
		at.Function.Description = t.Description
		at.Function.Parameters = t.Parameters
		apiTools = append(apiTools, at)
	}

	payload := map[string]any{
		"model":       c.Model,
		"messages":    msgs,
		"temperature": 0.4,
	}
	if len(apiTools) > 0 {
		payload["tools"] = apiTools
		payload["tool_choice"] = "auto"
	}
	if c.ReasoningEffort != "" {
		payload["reasoning_effort"] = c.ReasoningEffort
	}
	return payload
}

func (c *Client) Respond(ctx context.Context, convo []agent.Message, tools []agent.Tool) (agent.Reply, error) {
	payload := c.payload(convo, tools)

	body, err := json.Marshal(payload)
	if err != nil {
		return agent.Reply{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return agent.Reply{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("api-subscription-key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return agent.Reply{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return agent.Reply{}, fmt.Errorf("openai: %s: %s", resp.Status, string(b))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content   string        `json:"content"`
				ToolCalls []apiToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return agent.Reply{}, err
	}
	if len(out.Choices) == 0 {
		return agent.Reply{}, fmt.Errorf("openai: empty choices")
	}

	msg := out.Choices[0].Message
	reply := agent.Reply{Text: msg.Content}
	if len(msg.ToolCalls) > 0 {
		tc := msg.ToolCalls[0]
		reply.ToolCall = &agent.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: json.RawMessage(tc.Function.Arguments),
		}
	}
	return reply, nil
}

func (c *Client) Stream(ctx context.Context, convo []agent.Message, tools []agent.Tool, onText func(string) error) (agent.Reply, error) {
	payload := c.payload(convo, tools)
	payload["stream"] = true

	body, err := json.Marshal(payload)
	if err != nil {
		return agent.Reply{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return agent.Reply{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("api-subscription-key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return agent.Reply{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return agent.Reply{}, fmt.Errorf("openai: %s: %s", resp.Status, string(b))
	}
	if onText == nil {
		onText = func(string) error { return nil }
	}

	type acc struct {
		id   string
		name string
		args strings.Builder
	}
	calls := map[int]*acc{}
	var order []int
	var text strings.Builder

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				text.WriteString(choice.Delta.Content)
				if err := onText(choice.Delta.Content); err != nil {
					return agent.Reply{}, err
				}
			}
			for _, tc := range choice.Delta.ToolCalls {
				a := calls[tc.Index]
				if a == nil {
					a = &acc{}
					calls[tc.Index] = a
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					a.id = tc.ID
				}
				if tc.Function.Name != "" {
					a.name += tc.Function.Name
				}
				a.args.WriteString(tc.Function.Arguments)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return agent.Reply{}, err
	}

	reply := agent.Reply{Text: text.String()}
	if len(order) > 0 {
		a := calls[order[0]]
		id := a.id
		if id == "" {
			id = "call_" + a.name
		}
		args := json.RawMessage(a.args.String())
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		reply.ToolCall = &agent.ToolCall{ID: id, Name: a.name, Args: args}
	}
	return reply, nil
}
