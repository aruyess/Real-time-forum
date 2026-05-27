package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"forum/internal/db"
	"forum/internal/session"
	wsint "forum/internal/ws"

	"github.com/gorilla/websocket"
)

type WSHandler struct {
	DB  *sql.DB
	Hub *wsint.Hub
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Same-origin in development. Replace with a stricter check for production
	// deployments behind a real domain.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handle upgrades the HTTP request to a WebSocket and hands the connection
// off to the hub. Authentication uses the same session cookie as REST.
func (h *WSHandler) Handle(w http.ResponseWriter, r *http.Request) {
	s, err := session.FromRequest(r.Context(), h.DB, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	user, err := db.GetUserByID(r.Context(), h.DB, s.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrader has already written a response; just log.
		log.Printf("ws upgrade [%s]: %v", user.Nickname, err)
		return
	}

	client := wsint.NewClient(h.Hub, conn, user.ID, user.Nickname)
	go client.Start()
}
