package ws

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/coder/websocket"
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
		if err := c.Write(ctx, typ, data); err != nil {
			return err
		}
	}
}
