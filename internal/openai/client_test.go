package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chinmay/switchboard/internal/agent"
)

func TestRespondParsesToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Errorf("missing or wrong auth header")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		if body["model"] != "gpt-4o-mini" {
			t.Errorf("unexpected model: %v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"book","arguments":"{\"date\":\"2026-10-30\"}"}}]}}]}`))
	}))
	defer srv.Close()

	c := &Client{APIKey: "test", BaseURL: srv.URL, Model: "gpt-4o-mini", HTTP: srv.Client()}
	reply, err := c.Respond(context.Background(), []agent.Message{{Role: "user", Content: "hi"}}, []agent.Tool{{Name: "book"}})
	if err != nil {
		t.Fatal(err)
	}
	if reply.ToolCall == nil || reply.ToolCall.Name != "book" {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	var args map[string]string
	if err := json.Unmarshal(reply.ToolCall.Args, &args); err != nil {
		t.Fatal(err)
	}
	if args["date"] != "2026-10-30" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestStreamParsesText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\" there.\"}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := &Client{APIKey: "test", BaseURL: srv.URL, Model: "test", HTTP: srv.Client()}
	var got strings.Builder
	reply, err := c.Stream(context.Background(), []agent.Message{{Role: "user", Content: "hi"}}, nil, func(s string) error {
		got.WriteString(s)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.Text != "Hello there." || got.String() != "Hello there." {
		t.Fatalf("reply=%q streamed=%q", reply.Text, got.String())
	}
}

func TestStreamAssemblesToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"book\",\"arguments\":\"{\\\"date\\\":\"}}]}}]}\n\n")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"2026-10-30\\\"}\"}}]}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := &Client{APIKey: "test", BaseURL: srv.URL, Model: "test", HTTP: srv.Client()}
	reply, err := c.Stream(context.Background(), []agent.Message{{Role: "user", Content: "book"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reply.ToolCall == nil || reply.ToolCall.Name != "book" {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	var args map[string]string
	if err := json.Unmarshal(reply.ToolCall.Args, &args); err != nil {
		t.Fatal(err)
	}
	if args["date"] != "2026-10-30" {
		t.Fatalf("unexpected args: %v", args)
	}
}
