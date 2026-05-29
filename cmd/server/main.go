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

	"forum/internal/server"
)

func main() {
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
