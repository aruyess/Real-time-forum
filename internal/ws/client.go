package ws

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// Timing constants for keep-alive + slow-client detection.
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10 // must be < pongWait
	maxMessageSize = 2048                // we don't accept big inbound messages
)

// Client owns one WebSocket connection. Reads run in readPump and writes
// (including pings) run in writePump. The hub talks to writePump via the
// send channel.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   string
	nickname string
}

func NewClient(hub *Hub, conn *websocket.Conn, userID, nickname string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 32),
		userID:   userID,
		nickname: nickname,
	}
}

// Start registers the client with the hub and launches the read/write loops.
// readPump runs in the calling goroutine; writePump runs in its own.
// The hub-issued "user.online" event fires synchronously inside register.
func (c *Client) Start() {
	c.hub.register <- c
	go c.writePump()
	c.readPump()
}

// readPump reads messages from the client until it errors or closes.
// Inbound WS frames are unused — chat goes through the REST POST endpoint
// and the server broadcasts results back; this pump exists only to detect
// disconnects (close frames, pong timeouts) so the hub can mark the user
// offline.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
				websocket.CloseNormalClosure,
			) {
				log.Printf("ws read [%s]: %v", c.nickname, err)
			}
			return
		}
	}
}

// writePump drains the send channel, pinging periodically so idle connections
// aren't dropped by intermediaries.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
