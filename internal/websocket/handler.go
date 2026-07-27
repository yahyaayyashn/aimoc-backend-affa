package wshub

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
)

func Serve(c *websocket.Conn) {
	channel := c.Params("channel")
	if channel == "" {
		channel = c.Query("channel", "dashboard")
	}
	cl := &Client{ID: uuid.NewString(), Channel: channel, Send: make(chan []byte, 32)}
	Default.Register(cl)
	defer Default.Unregister(cl)

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case msg, ok := <-cl.Send:
			if !ok {
				return
			}
			if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}
}
