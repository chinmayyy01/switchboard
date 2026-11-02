package ws

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"go.opentelemetry.io/otel/attribute"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/ai"
	"github.com/chinmay/switchboard/internal/booking"
	"github.com/chinmay/switchboard/internal/guard"
	"github.com/chinmay/switchboard/internal/hub"
	"github.com/chinmay/switchboard/internal/inference"
	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/metrics"
	"github.com/chinmay/switchboard/internal/obs"
	"github.com/chinmay/switchboard/internal/openai"
	"github.com/chinmay/switchboard/internal/store"
	"github.com/chinmay/switchboard/internal/tenant"
)

const frameQueue = 256

var acceptOpts = &websocket.AcceptOptions{
	OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	if r.URL.Query().Get("token") == token {
		return true
	}
	return r.Header.Get("X-Supervisor-Token") == token
}

func Handler(h *hub.Hub, st *store.Store, reg *BridgeRegistry, jobsReg *jobs.Registry, tenants *tenant.Registry, base context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			tenantID = "rivera"
		}
		t, ok := tenants.Get(tenantID)
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}

		c, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			slog.Error("ws accept", "err", err)
			return
		}
		defer c.CloseNow()
		if err := run(base, c, h, st, reg, jobsReg, t); err != nil {
			slog.Error("ws session ended", "err", err)
		}
	}
}

func SupervisorHandler(h *hub.Hub, token string, base context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			slog.Error("supervisor accept", "err", err)
			return
		}
		defer c.CloseNow()

		id, ch := h.Subscribe()
		defer h.Unsubscribe(id)

		for {
			select {
			case <-base.Done():
				return
			case snap := <-ch:
				b, err := json.Marshal(snap)
				if err != nil {
					continue
				}
				if err := c.Write(base, websocket.MessageText, b); err != nil {
					return
				}
			}
		}
	}
}

func SupervisorCallHandler(reg *BridgeRegistry, h *hub.Hub, token string, base context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := r.URL.Query().Get("id")
		br, ok := reg.get(id)
		if !ok {
			http.Error(w, "no such call", http.StatusNotFound)
			return
		}
		c, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			slog.Error("bridge accept", "err", err)
			return
		}
		defer c.CloseNow()

		h.TakeOver(id)
		cw := &connWriter{ctx: base, c: c}
		br.setSupervisor(cw)
		defer br.clearSupervisor(cw)

		for {
			typ, data, err := c.Read(base)
			if err != nil {
				return
			}
			if typ == websocket.MessageBinary {
				br.toCaller(pcmFromBytes(data))
			}
		}
	}
}

type callMsg struct {
	control string
	lang    string
	text    string
	pcm     []int16
}

func run(ctx context.Context, c *websocket.Conn, h *hub.Hub, st *store.Store, reg *BridgeRegistry, jobsReg *jobs.Registry, t tenant.Tenant) error {
	c.SetReadLimit(64 * 1024)

	callID := newCallID()
	ctx, span := obs.Tracer().Start(ctx, "call")
	defer span.End()
	span.SetAttributes(attribute.String("call.id", callID), attribute.String("tenant", t.ID))
	callLog := slog.With("call_id", callID, "tenant", t.ID)
	callLog.Info("call.start")

	startedAt := time.Now()
	h.Register(callID, t.ID)
	metrics.CallStarted()
	defer h.Unregister(callID)

	br := reg.getOrCreate(callID)
	defer reg.remove(callID)

	var sel agent.Selector = agent.NewFakeSelector()
	realSel, isReal := inference.NewFromEnv()
	if isReal {
		sel = realSel
	}

	w := &connWriter{ctx: ctx, c: c}
	br.setCaller(w)
	defer br.clearCaller(w)

	var kb booking.Answerer
	if c := ai.NewFromEnv(); c != nil {
		kb = c
	}
	jobStore := jobsReg.For(t.ID)
	jobStore.SetWindows(t.Windows)

	var brain agent.Brain
	if o := openai.NewSarvamFromEnv(); o != nil {
		brain = booking.NewLLM(jobStore, kb, o, t.ID, t.Name, t.Services)
	} else if o := openai.NewFromEnv(); o != nil {
		brain = booking.NewLLM(jobStore, kb, o, t.ID, t.Name, t.Services)
	} else if isReal && os.Getenv("BRAIN") == "llm" {
		brain = booking.NewLLM(jobStore, kb, sel.Select("").LLM, t.ID, t.Name, t.Services)
	} else if isReal && os.Getenv("BRAIN") == "hybrid" {
		brain = booking.NewHybrid(booking.New(jobStore, kb, t.ID, t.Name), sel.Select("").LLM)
	} else {
		brain = booking.New(jobStore, kb, t.ID, t.Name)
	}

	sess := agent.NewSession(sel, brain, wsSink{w: w, hub: h, callID: callID})
	h.SetControl(callID, func() { sess.SetHumanOwned(true) })

	msgs := make(chan callMsg, frameQueue)
	var dropped atomic.Int64
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for m := range msgs {
			if m.pcm != nil {
				if err := sess.OnAudio(ctx, m.pcm); err != nil {
					slog.Error("pipeline", "call_id", callID, "err", err)
					return
				}
				continue
			}
			switch m.control {
			case "call.start":
				sess.SetLanguage(m.lang)
				if m.lang != "" {
					h.SetLang(callID, m.lang)
					callLog.Info("call.lang", "lang", m.lang)
				}
				lang := m.lang
				if lang == "" {
					lang = "en"
				}
				greeting := fmt.Sprintf(
					"Hi, this is Alex at %s. I can book a visit, reschedule or cancel one, answer questions, or connect you to a person. What can I help you with?",
					t.Name,
				)
				if strings.HasPrefix(lang, "hi") {
					greeting = fmt.Sprintf(
						"नमस्ते, मैं %s से Alex हूँ। मैं विज़िट बुक कर सकता हूँ, रीशेड्यूल या कैंसिल कर सकता हूँ, सवालों के जवाब दे सकता हूँ, या किसी व्यक्ति से जोड़ सकता हूँ। मैं कैसे मदद कर सकता हूँ?",
						t.Name,
					)
				}
				if err := sess.Greet(ctx, greeting, lang); err != nil {
					slog.Error("greeting", "call_id", callID, "err", err)
				}
			case "interrupt":
				sess.Interrupt()
				callLog.Info("interrupt")
			case "user.text":
				if err := sess.HandleText(ctx, m.text, m.lang); err != nil {
					slog.Error("pipeline", "call_id", callID, "err", err)
					return
				}
			}
		}
	}()
	defer func() {
		close(msgs)
		<-workerDone

		metrics.CallEnded()
		metrics.Dropped(int(dropped.Load()))
		sum := sess.Summary()
		turns := make([]store.Turn, 0, 8)
		for _, t := range h.Transcript(callID) {
			turns = append(turns, store.Turn{Role: t.Role, Text: t.Text})
		}
		st.Save(store.Record{
			ID:         callID,
			Tenant:     t.ID,
			Lang:       sum.Lang,
			Turns:      sum.Turns,
			Escalated:  sum.Escalated,
			Reason:     sum.Reason,
			Signal:     sum.Signal,
			Transcript: turns,
			Summary:    store.Summarize(turns, sum.Escalated, sum.Reason),
			StartedAt:  startedAt,
			EndedAt:    time.Now(),
		})
		if n := dropped.Load(); n > 0 {
			slog.Warn("backpressure", "call_id", callID, "dropped", n)
		}
		callLog.Info("call.end",
			"lang", sum.Lang,
			"turns", sum.Turns,
			"escalated", sum.Escalated,
			"reason", sum.Reason,
			"dropped", dropped.Load(),
		)
	}()

	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			switch websocket.CloseStatus(err) {
			case websocket.StatusNormalClosure, websocket.StatusGoingAway:
				return nil
			}
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		switch typ {
		case websocket.MessageText:
			var msg struct {
				Type string `json:"type"`
				Lang string `json:"lang"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.Type == "ping" {
				if err := w.write(websocket.MessageText, data); err != nil {
					return err
				}
				continue
			}
			if msg.Type == "interrupt" {
				sess.CancelSpeech()
			}
			push(msgs, callMsg{control: msg.Type, lang: msg.Lang, text: msg.Text}, &dropped)
		case websocket.MessageBinary:
			pcm := pcmFromBytes(data)
			if br.hasSupervisor() {
				br.toSupervisor(pcm)
			}
			push(msgs, callMsg{pcm: pcm}, &dropped)
		}
	}
}

func push(msgs chan callMsg, m callMsg, dropped *atomic.Int64) {
	select {
	case msgs <- m:
		return
	default:
	}
	if m.control != "" {
		select {
		case <-msgs:
			dropped.Add(1)
		default:
		}
	}
	select {
	case msgs <- m:
	default:
		dropped.Add(1)
	}
}

func newCallID() string {
	var b [4]byte
	rand.Read(b[:])
	return "call-" + hex.EncodeToString(b[:])
}

type connWriter struct {
	mu  sync.Mutex
	ctx context.Context
	c   *websocket.Conn
}

func (w *connWriter) write(typ websocket.MessageType, b []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.c.Write(w.ctx, typ, b)
}

func (w *connWriter) writePCM(pcm []int16) error {
	b := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return w.write(websocket.MessageBinary, b)
}

type wsSink struct {
	w      *connWriter
	hub    *hub.Hub
	callID string
}

func (s wsSink) EventJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if m, ok := v.(map[string]any); ok {
		switch m["type"] {
		case "stt.final":
			if t, ok := m["text"].(string); ok {
				s.hub.SetText(s.callID, t)
				s.hub.AddTurn(s.callID, "caller", guard.Redact(t))
			}
			if l, ok := m["lang"].(string); ok {
				s.hub.SetLang(s.callID, l)
			}
		case "agent.state":
			if st, ok := m["state"].(string); ok {
				s.hub.SetState(s.callID, st)
			}
		case "agent.text":
			if t, ok := m["text"].(string); ok {
				s.hub.AddTurn(s.callID, "agent", t)
			}
		case "tool.call":
			slog.Info("tool.call", "call_id", s.callID, "name", m["name"])
		case "escalation":
			s.hub.SetEscalated(s.callID, true)
			if reason, ok := m["reason"].(string); ok {
				s.hub.SetReason(s.callID, reason)
				slog.Info("escalation", "call_id", s.callID, "reason", reason)
			}
			s.hub.SetState(s.callID, "escalated")
		case "guardrail":
			slog.Warn("guardrail", "call_id", s.callID, "kind", m["kind"])
		}
	}
	return s.w.write(websocket.MessageText, b)
}

func (s wsSink) Audio(pcm []int16) error {
	const chunkSamples = 1600
	for start := 0; start < len(pcm); start += chunkSamples {
		end := start + chunkSamples
		if end > len(pcm) {
			end = len(pcm)
		}
		part := pcm[start:end]
		b := make([]byte, len(part)*2)
		for i, v := range part {
			binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
		}
		if err := s.w.write(websocket.MessageBinary, b); err != nil {
			return err
		}
	}
	return nil
}

func pcmFromBytes(b []byte) []int16 {
	n := len(b) / 2
	pcm := make([]int16, n)
	for i := 0; i < n; i++ {
		pcm[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return pcm
}
