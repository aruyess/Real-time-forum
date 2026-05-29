// Package server owns the HTTP server bootstrap: configuration, database
// setup, uploads directory, routes, and WebSocket hub.
package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"forum/internal/db"
	"forum/internal/routes"
	"forum/internal/ws"
)

// Run starts the forum HTTP server using environment-based configuration.
func Run() error {
	dbPath := os.Getenv("FORUM_DB")
	if dbPath == "" {
		dbPath = "forum.db"
	}
	addr := os.Getenv("FORUM_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// Uploads live alongside the DB file so that a single data volume in
	// production captures everything: SQLite + image attachments.
	uploadsDir := filepath.Join(filepath.Dir(dbPath), "uploads")
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return err
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()
	log.Printf("db: opened %s, schema applied", dbPath)

	hub := ws.NewHub()
	go hub.Run()

	mux := http.NewServeMux()
	routes.Register(mux, database, hub, uploadsDir)

	log.Printf("listening on http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}
