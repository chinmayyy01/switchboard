package ws

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/hub"
	"github.com/chinmay/switchboard/internal/inference"
	"github.com/chinmay/switchboard/internal/metrics"
	"github.com/chinmay/switchboard/internal/store"
)

const frameQueue = 128

var acceptOpts = &websocket.AcceptOptions{
	OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
}

func Handler(h *hub.Hub, st *store.Store, base context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			log.Printf("ws accept: %v", err)
			return
		}
		defer c.CloseNow()
		if err := run(base, c, h, st); err != nil {
			log.Printf("ws session ended: %v", err)
		}
	}
}

func SupervisorHandler(h *hub.Hub, base context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			log.Printf("supervisor accept: %v", err)
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

type callMsg struct {
	control string
	lang    string
	pcm     []int16
}

func run(ctx context.Context, c *websocket.Conn, h *hub.Hub, st *store.Store) error {
	c.SetReadLimit(64 * 1024)

	callID := newCallID()
	startedAt := time.Now()
	h.Register(callID)
	metrics.CallStarted()
	defer h.Unregister(callID)

	var sel agent.Selector = agent.NewFakeSelector()
	if real, ok := inference.NewFromEnv(); ok {
		sel = real
	}

	w := &connWriter{ctx: ctx, c: c}
	sess := agent.NewSession(sel, agent.FakeBrain{}, wsSink{w: w, hub: h, callID: callID})
	h.SetControl(callID, func() { sess.SetHumanOwned(true) })

	msgs := make(chan callMsg, frameQueue)
	var dropped atomic.Int64
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for m := range msgs {
			if m.pcm != nil {
				if err := sess.OnAudio(ctx, m.pcm); err != nil {
					log.Printf("call %s: pipeline: %v", callID, err)
					return
				}
				continue
			}
			switch m.control {
			case "call.start":
				sess.SetLanguage(m.lang)
				if m.lang != "" {
					h.SetLang(callID, m.lang)
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
		st.Save(store.Record{
			ID:        callID,
			Lang:      sum.Lang,
			Turns:     sum.Turns,
			Escalated: sum.Escalated,
			Reason:    sum.Reason,
			Signal:    sum.Signal,
			StartedAt: startedAt,
			EndedAt:   time.Now(),
		})
		if n := dropped.Load(); n > 0 {
			log.Printf("call %s: dropped %d frames (backpressure)", callID, n)
		}
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
			push(msgs, callMsg{control: msg.Type, lang: msg.Lang}, &dropped)
		case websocket.MessageBinary:
			push(msgs, callMsg{pcm: pcmFromBytes(data)}, &dropped)
		}
	}
}

func push(msgs chan callMsg, m callMsg, dropped *atomic.Int64) {
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
				s.hub.AddTurn(s.callID, "caller", t)
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
		case "escalation":
			s.hub.SetEscalated(s.callID, true)
			if reason, ok := m["reason"].(string); ok {
				s.hub.SetReason(s.callID, reason)
			}
			s.hub.SetState(s.callID, "escalated")
		}
	}
	return s.w.write(websocket.MessageText, b)
}

func (s wsSink) Audio(pcm []int16) error {
	b := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return s.w.write(websocket.MessageBinary, b)
}

func pcmFromBytes(b []byte) []int16 {
	n := len(b) / 2
	pcm := make([]int16, n)
	for i := 0; i < n; i++ {
		pcm[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return pcm
}
