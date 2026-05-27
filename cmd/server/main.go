// Command server is the entry point for the real-time forum HTTP server.
//
// Run from the project root so the relative paths to the web/ directory
// resolve correctly:
//
//	go run ./cmd/server
//	go build -o forum ./cmd/server && ./forum
//
// Configuration via environment variables:
//
//	FORUM_DB   path to the SQLite file (default: forum.db, relative to cwd)
//	FORUM_ADDR listen address          (default: :8080)
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"forum/internal/db"
	"forum/internal/routes"
	"forum/internal/ws"
)

func main() {
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
		log.Fatalf("uploads dir: %v", err)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()
	log.Printf("db: opened %s, schema applied", dbPath)

	hub := ws.NewHub()
	go hub.Run()

	mux := http.NewServeMux()
	routes.Register(mux, database, hub, uploadsDir)

	log.Printf("listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
