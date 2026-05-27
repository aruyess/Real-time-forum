package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"forum/internal/db"
	"forum/internal/models"

	"github.com/google/uuid"
)

type Posts struct {
	DB *sql.DB
}

// ---------- GET /api/posts ----------
//
// Optional query params:
//   category=<name>  show only posts tagged with this category
//   author=me        show only the authenticated user's own posts
//   liked=me         show only posts the authenticated user has liked
//
// author=me / liked=me require auth; the rest works for anonymous viewers.

func (p *Posts) List(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := authedUserID(p.DB, r) // empty for anonymous; intentional

	// ?category=a&category=b&category=c → OR-filter across the three names.
	// Empty values (e.g. trailing &category=) are skipped to keep the IN-list clean.
	var cats []string
	for _, c := range r.URL.Query()["category"] {
		if c = strings.TrimSpace(c); c != "" {
			cats = append(cats, c)
		}
	}

	f := db.PostListFilter{
		Categories: cats,
		ViewerID:   viewerID,
		Limit:      50,
	}
	switch author := r.URL.Query().Get("author"); author {
	case "":
		// no filter
	case "me":
		if viewerID == "" {
			writeError(w, http.StatusUnauthorized, "log in to filter by your own posts")
			return
		}
		f.AuthorID = viewerID
	default:
		// arbitrary user id (e.g. from a profile page)
		f.AuthorID = author
	}
	if r.URL.Query().Get("liked") == "me" {
		if viewerID == "" {
			writeError(w, http.StatusUnauthorized, "log in to filter by liked posts")
			return
		}
		f.LikedBy = viewerID
	}

	items, err := db.ListPosts(r.Context(), p.DB, f)
	if err != nil {
		log.Printf("list posts: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load posts")
		return
	}
	if items == nil {
		items = []models.Post{}
	}
	writeJSON(w, http.StatusOK, items)
}

// ---------- POST /api/posts ----------

type createPostReq struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	CategoryIDs []int  `json:"categoryIds"`
}

func (p *Posts) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(p.DB, w, r)
	if !ok {
		return
	}

	var req createPostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)

	if msg := validatePost(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	post := &models.Post{
		ID:      uuid.NewString(),
		UserID:  userID,
		Title:   req.Title,
		Content: req.Content,
	}
	if err := db.CreatePost(r.Context(), p.DB, post, req.CategoryIDs); err != nil {
		if errors.Is(err, db.ErrInvalidCategory) {
			writeError(w, http.StatusBadRequest, "invalid category")
			return
		}
		log.Printf("create post: %v", err)
		writeError(w, http.StatusInternalServerError, "could not create post")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": post.ID})
}

func validatePost(r *createPostReq) string {
	if n := len(r.Title); n < 3 || n > 200 {
		return "title must be 3–200 characters"
	}
	if n := len(r.Content); n < 1 || n > 5000 {
		return "content must be 1–5000 characters"
	}
	if len(r.CategoryIDs) == 0 {
		return "at least one category is required"
	}
	if len(r.CategoryIDs) > 5 {
		return "no more than 5 categories per post"
	}
	return ""
}

// ---------- PUT /api/posts/{id} ----------

func (p *Posts) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(p.DB, w, r)
	if !ok {
		return
	}

	var req createPostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)

	if msg := validatePost(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	postID := r.PathValue("id")
	err := db.UpdatePost(r.Context(), p.DB, postID, userID,
		req.Title, req.Content, req.CategoryIDs)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "post not found or not yours")
		case errors.Is(err, db.ErrInvalidCategory):
			writeError(w, http.StatusBadRequest, "invalid category")
		default:
			log.Printf("update post: %v", err)
			writeError(w, http.StatusInternalServerError, "could not update post")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- DELETE /api/posts/{id} ----------

func (p *Posts) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(p.DB, w, r)
	if !ok {
		return
	}

	postID := r.PathValue("id")
	if err := db.DeletePost(r.Context(), p.DB, postID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "post not found or not yours")
			return
		}
		log.Printf("delete post: %v", err)
		writeError(w, http.StatusInternalServerError, "could not delete post")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- GET /api/posts/{id} ----------

func (p *Posts) Get(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := authedUserID(p.DB, r) // anonymous OK
	id := r.PathValue("id")
	post, err := db.GetPost(r.Context(), p.DB, id, viewerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		log.Printf("get post: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load post")
		return
	}
	writeJSON(w, http.StatusOK, post)
}

// ---------- GET /api/categories ----------

func (p *Posts) Categories(w http.ResponseWriter, r *http.Request) {
	cats, err := db.ListCategories(r.Context(), p.DB)
	if err != nil {
		log.Printf("list categories: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load categories")
		return
	}
	if cats == nil {
		cats = []models.Category{}
	}
	writeJSON(w, http.StatusOK, cats)
}

// ---------- GET /api/posts/{id}/comments ----------

func (p *Posts) ListComments(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := authedUserID(p.DB, r) // anonymous OK
	postID := r.PathValue("id")
	items, err := db.ListCommentsForPost(r.Context(), p.DB, postID, viewerID, 200)
	if err != nil {
		log.Printf("list comments: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load comments")
		return
	}
	if items == nil {
		items = []models.Comment{}
	}
	writeJSON(w, http.StatusOK, items)
}

// ---------- POST /api/posts/{id}/comments ----------

type addCommentReq struct {
	Content string `json:"content"`
}

func (p *Posts) AddComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(p.DB, w, r)
	if !ok {
		return
	}
	postID := r.PathValue("id")

	var req addCommentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if n := len(req.Content); n < 1 || n > 2000 {
		writeError(w, http.StatusBadRequest, "comment must be 1–2000 characters")
		return
	}

	c := &models.Comment{
		ID:      uuid.NewString(),
		PostID:  postID,
		UserID:  userID,
		Content: req.Content,
	}
	if err := db.CreateComment(r.Context(), p.DB, c); err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		log.Printf("create comment: %v", err)
		writeError(w, http.StatusInternalServerError, "could not create comment")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": c.ID})
}

// ---------- PUT /api/comments/{id} ----------

func (p *Posts) UpdateComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(p.DB, w, r)
	if !ok {
		return
	}
	commentID := r.PathValue("id")

	var req addCommentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if n := len(req.Content); n < 1 || n > 2000 {
		writeError(w, http.StatusBadRequest, "comment must be 1–2000 characters")
		return
	}

	if err := db.UpdateComment(r.Context(), p.DB, commentID, userID, req.Content); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "comment not found or not yours")
			return
		}
		log.Printf("update comment: %v", err)
		writeError(w, http.StatusInternalServerError, "could not update comment")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- DELETE /api/comments/{id} ----------

func (p *Posts) DeleteComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(p.DB, w, r)
	if !ok {
		return
	}
	commentID := r.PathValue("id")
	if err := db.DeleteComment(r.Context(), p.DB, commentID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "comment not found or not yours")
			return
		}
		log.Printf("delete comment: %v", err)
		writeError(w, http.StatusInternalServerError, "could not delete comment")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
