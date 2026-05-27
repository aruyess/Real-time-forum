package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forum/internal/db"
	"forum/internal/models"
	"forum/internal/ws"

	"github.com/google/uuid"
)

type Messages struct {
	DB  *sql.DB
	Hub *ws.Hub
}

// ---------- GET /api/messages?with=<userId>&before=<ts>&limit=<n> ----------

func (m *Messages) List(w http.ResponseWriter, r *http.Request) {
	selfID, ok := requireAuth(m.DB, w, r)
	if !ok {
		return
	}

	other := r.URL.Query().Get("with")
	if other == "" {
		writeError(w, http.StatusBadRequest, "'with' parameter is required")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	var beforeTime *time.Time
	if b := r.URL.Query().Get("before"); b != "" {
		t, err := time.Parse(time.RFC3339Nano, b)
		if err != nil {
			t, err = time.Parse(time.RFC3339, b)
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'before' timestamp")
			return
		}
		beforeTime = &t
	}

	msgs, err := db.ListMessages(r.Context(), m.DB, selfID, other, beforeTime, limit)
	if err != nil {
		log.Printf("list messages: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load messages")
		return
	}
	if msgs == nil {
		msgs = []models.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// ---------- GET /api/chats/{peerId}/read-state ----------

// ReadState returns when the peer last read THIS user's messages, so the
// chat UI can render initial read-receipt ticks on outgoing bubbles
// without waiting for a chat.read WS event.
func (m *Messages) ReadState(w http.ResponseWriter, r *http.Request) {
	selfID, ok := requireAuth(m.DB, w, r)
	if !ok {
		return
	}
	peerID := r.PathValue("peerId")
	t, err := db.PeerLastReadAt(r.Context(), m.DB, selfID, peerID)
	if err != nil {
		log.Printf("read-state: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load read state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peerLastReadAt": t})
}

// ---------- POST /api/chats/{peerId}/read ----------

// MarkRead records that the caller has read everything from peerID up to
// "now" and fans out a chat.read WS event so the peer can flip their own
// outgoing messages to "read" in the chat UI.
func (m *Messages) MarkRead(w http.ResponseWriter, r *http.Request) {
	selfID, ok := requireAuth(m.DB, w, r)
	if !ok {
		return
	}
	peerID := r.PathValue("peerId")
	if peerID == "" || peerID == selfID {
		writeError(w, http.StatusBadRequest, "invalid peer id")
		return
	}
	ts, err := db.MarkChatRead(r.Context(), m.DB, selfID, peerID)
	if err != nil {
		log.Printf("mark chat read: %v", err)
		writeError(w, http.StatusInternalServerError, "could not mark read")
		return
	}

	// Tell the peer "your messages to me have been read up to <ts>" so
	// they can render read-receipts on their own outgoing bubbles.
	if m.Hub != nil {
		payload, err := json.Marshal(struct {
			Type       string    `json:"type"`
			ReaderID   string    `json:"readerId"`
			LastReadAt time.Time `json:"lastReadAt"`
		}{
			Type:       "chat.read",
			ReaderID:   selfID,
			LastReadAt: ts,
		})
		if err == nil {
			m.Hub.SendToUsers([]string{peerID}, payload)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- POST /api/messages ----------

type sendMessageReq struct {
	To       string `json:"to"`
	Content  string `json:"content"`
	ImageURL string `json:"imageUrl"`
}

func (m *Messages) Send(w http.ResponseWriter, r *http.Request) {
	selfID, ok := requireAuth(m.DB, w, r)
	if !ok {
		return
	}

	var req sendMessageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	req.ImageURL = strings.TrimSpace(req.ImageURL)

	if req.To == "" {
		writeError(w, http.StatusBadRequest, "'to' is required")
		return
	}
	// A message must carry at least text or an image.
	if req.Content == "" && req.ImageURL == "" {
		writeError(w, http.StatusBadRequest, "message must have text or an image")
		return
	}
	if len(req.Content) > 2000 {
		writeError(w, http.StatusBadRequest, "message must be at most 2000 characters")
		return
	}
	if req.ImageURL != "" && !strings.HasPrefix(req.ImageURL, "/uploads/") {
		writeError(w, http.StatusBadRequest, "invalid image URL")
		return
	}
	if req.To == selfID {
		writeError(w, http.StatusBadRequest, "cannot message yourself")
		return
	}

	sender, err := db.GetUserByID(r.Context(), m.DB, selfID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	msg := &models.Message{
		ID:             uuid.NewString(),
		SenderID:       selfID,
		ReceiverID:     req.To,
		SenderNickname: sender.Nickname,
		Content:        req.Content,
		ImageURL:       req.ImageURL,
	}
	if err := db.CreateMessage(r.Context(), m.DB, msg); err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "recipient not found")
			return
		}
		log.Printf("create message: %v", err)
		writeError(w, http.StatusInternalServerError, "could not send message")
		return
	}

	// Push the message in real-time to both sides. The sender deduplicates
	// by id so their own POST response doesn't double-render with the echo.
	if m.Hub != nil {
		payload, err := json.Marshal(struct {
			Type    string          `json:"type"`
			Message *models.Message `json:"message"`
		}{
			Type:    "message.new",
			Message: msg,
		})
		if err == nil {
			m.Hub.SendToUsers([]string{msg.SenderID, msg.ReceiverID}, payload)
		} else {
			log.Printf("ws marshal message.new: %v", err)
		}
	}

	writeJSON(w, http.StatusCreated, msg)
}
