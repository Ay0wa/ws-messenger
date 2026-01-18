package ws

import (
	"context"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

type Client struct {
	Conn   *websocket.Conn
	ChatID uuid.UUID
	UserID uuid.UUID
	Send   chan []byte
}

func (c *Client) WriteLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.Send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_ = c.Conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
		}
	}
}
