package inference

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/chinmay/switchboard/internal/agent"
)

const defaultTimeout = 60 * time.Second

type WhisperSTT struct {
	BaseURL string
	HTTP    *http.Client
}

func (w *WhisperSTT) Transcribe(ctx context.Context, pcm []int16, lang string) (agent.Transcript, error) {
	url := w.BaseURL + "/stt"
	if lang != "" {
		url += "?lang=" + lang
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(pcmToBytes(pcm)))
	if err != nil {
		return agent.Transcript{}, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := client(w.HTTP).Do(req)
	if err != nil {
		return agent.Transcript{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return agent.Transcript{}, httpError("stt", resp)
	}
	var out struct {
		Text       string  `json:"text"`
		Lang       string  `json:"lang"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return agent.Transcript{}, err
	}
	return agent.Transcript{Text: out.Text, Lang: out.Lang, Confidence: out.Confidence}, nil
}

type PiperTTS struct {
	BaseURL string
	HTTP    *http.Client
}

func (p *PiperTTS) Synthesize(ctx context.Context, text, lang string) ([]int16, error) {
	body, err := json.Marshal(map[string]string{"text": text, "lang": lang})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/tts", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client(p.HTTP).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, httpError("tts", resp)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return bytesToPCM(raw), nil
}

type OllamaLLM struct {
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func (o *OllamaLLM) Respond(ctx context.Context, convo []agent.Message, tools []agent.Tool) (agent.Reply, error) {
	msgs := make([]chatMessage, len(convo))
	for i, m := range convo {
		msgs[i] = chatMessage{Role: m.Role, Content: m.Content}
	}
	payload := map[string]any{"model": o.Model, "stream": false, "messages": msgs}
	if len(tools) > 0 {
		payload["tools"] = toOllamaTools(tools)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return agent.Reply{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return agent.Reply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client(o.HTTP).Do(req)
	if err != nil {
		return agent.Reply{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return agent.Reply{}, httpError("llm", resp)
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return agent.Reply{}, err
	}
	reply := agent.Reply{Text: out.Message.Content}
	if len(out.Message.ToolCalls) > 0 {
		tc := out.Message.ToolCalls[0].Function
		reply.ToolCall = &agent.ToolCall{Name: tc.Name, Args: tc.Arguments}
	}
	return reply, nil
}

type Selector struct {
	STT agent.STT
	TTS agent.TTS
	LLM agent.LLM
}

func (s *Selector) Select(string) agent.Providers {
	return agent.Providers{STT: s.STT, TTS: s.TTS, LLM: s.LLM}
}

func NewFromEnv() (*Selector, bool) {
	inf := os.Getenv("INFERENCE_URL")
	if inf == "" {
		return nil, false
	}
	ollama := os.Getenv("OLLAMA_URL")
	if ollama == "" {
		ollama = "http://localhost:11434"
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "qwen3:1.7b"
	}
	return &Selector{
		STT: &WhisperSTT{BaseURL: inf, HTTP: &http.Client{Timeout: defaultTimeout}},
		TTS: &PiperTTS{BaseURL: inf, HTTP: &http.Client{Timeout: defaultTimeout}},
		LLM: &OllamaLLM{BaseURL: ollama, Model: model, HTTP: &http.Client{Timeout: defaultTimeout}},
	}, true
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaTool struct {
	Type     string         `json:"type"`
	Function ollamaFunction `json:"function"`
}

type ollamaFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func toOllamaTools(tools []agent.Tool) []ollamaTool {
	out := make([]ollamaTool, len(tools))
	for i, t := range tools {
		out[i] = ollamaTool{
			Type: "function",
			Function: ollamaFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return out
}

type chatResponse struct {
	Message struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		ToolCalls []struct {
			Function struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"message"`
}

func pcmToBytes(pcm []int16) []byte {
	b := make([]byte, len(pcm)*2)
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(s))
	}
	return b
}

func bytesToPCM(b []byte) []int16 {
	n := len(b) / 2
	pcm := make([]int16, n)
	for i := 0; i < n; i++ {
		pcm[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return pcm
}

func client(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return http.DefaultClient
}

func httpError(op string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("%s: %s: %s", op, resp.Status, string(b))
}
