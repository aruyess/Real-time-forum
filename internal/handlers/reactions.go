package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"forum/internal/db"
)

type Reactions struct {
	DB *sql.DB
}

type reactionReq struct {
	Value int `json:"value"` // -1, 0, or 1
}

// PUT /api/posts/{id}/reactions
func (h *Reactions) ForPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(h.DB, w, r)
	if !ok {
		return
	}
	h.toggle(w, r, "post", userID)
}

// PUT /api/comments/{id}/reactions
func (h *Reactions) ForComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(h.DB, w, r)
	if !ok {
		return
	}
	h.toggle(w, r, "comment", userID)
}

func (h *Reactions) toggle(w http.ResponseWriter, r *http.Request, kind, userID string) {
	var req reactionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Value != -1 && req.Value != 0 && req.Value != 1 {
		writeError(w, http.StatusBadRequest, "value must be -1, 0 or 1")
		return
	}
	targetID := r.PathValue("id")

	var (
		counts db.ReactionCounts
		err    error
	)
	if kind == "post" {
		counts, err = db.SetPostReaction(r.Context(), h.DB, targetID, userID, req.Value)
	} else {
		counts, err = db.SetCommentReaction(r.Context(), h.DB, targetID, userID, req.Value)
	}
	if err != nil {
		switch {
		case errors.Is(err, db.ErrPostNotFound):
			writeError(w, http.StatusNotFound, "post not found")
		case errors.Is(err, db.ErrCommentNotFound):
			writeError(w, http.StatusNotFound, "comment not found")
		default:
			log.Printf("set %s reaction: %v", kind, err)
			writeError(w, http.StatusInternalServerError, "could not save reaction")
		}
		return
	}
	writeJSON(w, http.StatusOK, counts)
}
