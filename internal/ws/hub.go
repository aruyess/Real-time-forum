package ws

import (
	"encoding/json"
	"log"
	"sync"
)

// Hub is the central bookkeeping for active WebSocket clients. It tracks
// which users are currently connected (possibly across multiple tabs) and
// broadcasts events to all connected clients.
//
// All state mutations happen inside Run() to avoid lock contention with the
// per-client goroutines; the only externally-locked field is byUser, which
// is also read by OnlineUsers().
type Hub struct {
	mu     sync.RWMutex
	byUser map[string]map[*Client]struct{} // userID -> connection set

	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	directed   chan directedMessage
}

// directedMessage is an envelope for sending a payload only to specific
// user IDs (each user may have multiple active client connections).
type directedMessage struct {
	userIDs []string
	payload []byte
}

func NewHub() *Hub {
	return &Hub{
		byUser:     make(map[string]map[*Client]struct{}),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		broadcast:  make(chan []byte, 128),
		directed:   make(chan directedMessage, 128),
	}
}

// Run is the hub's main loop. It owns all writes to byUser and the broadcast
// fan-out. Launch in a goroutine on server start.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			wasOffline := len(h.byUser[c.userID]) == 0
			if h.byUser[c.userID] == nil {
				h.byUser[c.userID] = make(map[*Client]struct{})
			}
			h.byUser[c.userID][c] = struct{}{}
			h.mu.Unlock()

			if wasOffline {
				h.broadcastJSON(Event{
					Type:     "user.online",
					UserID:   c.userID,
					Nickname: c.nickname,
				})
			}

		case c := <-h.unregister:
			h.mu.Lock()
			wentOffline := false
			if conns := h.byUser[c.userID]; conns != nil {
				delete(conns, c)
				if len(conns) == 0 {
					delete(h.byUser, c.userID)
					wentOffline = true
				}
			}
			h.mu.Unlock()
			// Close the send channel so the client's writePump exits cleanly.
			// Use a recover-style guard to avoid double-close panics if the
			// client was already torn down.
			func() {
				defer func() { _ = recover() }()
				close(c.send)
			}()

			if wentOffline {
				h.broadcastJSON(Event{Type: "user.offline", UserID: c.userID})
			}

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, conns := range h.byUser {
				for c := range conns {
					select {
					case c.send <- msg:
					default:
						// Send buffer full — drop this message for this client.
						// A persistent slow client will eventually be killed
						// by its own writePump deadline.
					}
				}
			}
			h.mu.RUnlock()

		case dm := <-h.directed:
			h.mu.RLock()
			for _, uid := range dm.userIDs {
				for c := range h.byUser[uid] {
					select {
					case c.send <- dm.payload:
					default:
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Event is a typed envelope for messages broadcast over WS. We keep a single
// struct with omitempty so we can extend it as new event types are added.
type Event struct {
	Type     string `json:"type"`
	UserID   string `json:"userId,omitempty"`
	Nickname string `json:"nickname,omitempty"`
}

func (h *Hub) broadcastJSON(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		log.Printf("ws marshal: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Printf("ws broadcast buffer full, dropping %s", e.Type)
	}
}

// SendToUsers fans the payload out only to the connections belonging to the
// given user IDs. Non-blocking — if the directed buffer is full the message
// is dropped (logged). Each user may have multiple connections (tabs).
func (h *Hub) SendToUsers(userIDs []string, payload []byte) {
	select {
	case h.directed <- directedMessage{userIDs: userIDs, payload: payload}:
	default:
		log.Printf("ws directed buffer full, dropping payload to %v", userIDs)
	}
}

// OnlineUsers returns a snapshot of currently-connected user IDs.
func (h *Hub) OnlineUsers() map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]bool, len(h.byUser))
	for uid := range h.byUser {
		out[uid] = true
	}
	return out
}
