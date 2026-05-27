// Package routes wires every HTTP route on a fresh ServeMux.
// It instantiates the handler structs from their dependencies (DB + WS hub)
// and registers static, REST and WebSocket endpoints in one place.
package routes

import (
	"database/sql"
	"net/http"
	"strings"

	"forum/internal/handlers"
	"forum/internal/ws"
)

// Register attaches every route the server knows about to mux.
// uploadsDir is the on-disk directory backing the /uploads/ static handler;
// upload requests write there and chat messages reference URLs under it.
func Register(mux *http.ServeMux, database *sql.DB, hub *ws.Hub, uploadsDir string) {
	auth := &handlers.Auth{DB: database}
	posts := &handlers.Posts{DB: database}
	users := &handlers.Users{DB: database, Hub: hub}
	messages := &handlers.Messages{DB: database, Hub: hub}
	reactions := &handlers.Reactions{DB: database}
	uploads := &handlers.Uploads{DB: database, Dir: uploadsDir}
	wsh := &handlers.WSHandler{DB: database, Hub: hub}

	// Static
	mux.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("web/css"))))
	mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("web/js"))))
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir))))

	// Auth
	mux.HandleFunc("POST /api/register", auth.Register)
	mux.HandleFunc("POST /api/login", auth.Login)
	mux.HandleFunc("POST /api/logout", auth.Logout)
	mux.HandleFunc("GET /api/me", auth.Me)
	mux.HandleFunc("PUT /api/me", auth.UpdateMe)

	// Posts & categories
	mux.HandleFunc("GET /api/categories", posts.Categories)
	mux.HandleFunc("GET /api/posts", posts.List)
	mux.HandleFunc("POST /api/posts", posts.Create)
	mux.HandleFunc("GET /api/posts/{id}", posts.Get)
	mux.HandleFunc("PUT /api/posts/{id}", posts.Update)
	mux.HandleFunc("DELETE /api/posts/{id}", posts.Delete)
	mux.HandleFunc("GET /api/posts/{id}/comments", posts.ListComments)
	mux.HandleFunc("POST /api/posts/{id}/comments", posts.AddComment)
	mux.HandleFunc("PUT /api/comments/{id}", posts.UpdateComment)
	mux.HandleFunc("DELETE /api/comments/{id}", posts.DeleteComment)

	// Reactions (like / dislike, value: -1, 0 or 1)
	mux.HandleFunc("PUT /api/posts/{id}/reactions", reactions.ForPost)
	mux.HandleFunc("PUT /api/comments/{id}/reactions", reactions.ForComment)

	// Users
	mux.HandleFunc("GET /api/users", users.List)
	mux.HandleFunc("GET /api/users/{id}", users.Get)

	// Messages
	mux.HandleFunc("GET /api/messages", messages.List)
	mux.HandleFunc("POST /api/messages", messages.Send)
	mux.HandleFunc("POST /api/chats/{peerId}/read", messages.MarkRead)
	mux.HandleFunc("GET /api/chats/{peerId}/read-state", messages.ReadState)

	// Image uploads — backs the optional image attachment on chat messages
	mux.HandleFunc("POST /api/uploads/image", uploads.Image)

	// WebSocket
	mux.HandleFunc("GET /ws", wsh.Handle)

	// SPA fallback. For unknown /api/* paths or wrong methods on known
	// /api/* paths, return JSON 404 instead of serving the HTML shell —
	// otherwise the frontend gets an HTML body when it expects JSON.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"endpoint not found"}`))
			return
		}
		http.ServeFile(w, r, "web/index.html")
	})
}
