package ws

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/coder/websocket"

	"github.com/chinmay/switchboard/internal/agent"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}
	defer c.CloseNow()

	if err := run(r.Context(), c); err != nil {
		log.Printf("ws session ended: %v", err)
	}
}

func run(ctx context.Context, c *websocket.Conn) error {
	c.SetReadLimit(64 * 1024)

	sess := agent.NewSession(
		agent.NewFakeSelector(),
		agent.FakeBrain{},
		wsSink{ctx: ctx, c: c},
	)

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
				if err := c.Write(ctx, websocket.MessageText, data); err != nil {
					return err
				}
				continue
			}
			if msg.Type == "call.start" {
				sess.SetLanguage(msg.Lang)
			}
		case websocket.MessageBinary:
			if err := sess.OnAudio(ctx, pcmFromBytes(data)); err != nil {
				return err
			}
		}
	}
}

type wsSink struct {
	ctx context.Context
	c   *websocket.Conn
}

func (w wsSink) EventJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.c.Write(w.ctx, websocket.MessageText, b)
}

func (w wsSink) Audio(pcm []int16) error {
	b := make([]byte, len(pcm)*2)
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(s))
	}
	return w.c.Write(w.ctx, websocket.MessageBinary, b)
}

func pcmFromBytes(b []byte) []int16 {
	n := len(b) / 2
	pcm := make([]int16, n)
	for i := 0; i < n; i++ {
		pcm[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return pcm
}
