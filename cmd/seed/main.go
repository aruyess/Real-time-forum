// Command seed inserts demo users and posts into the forum database.
//
// Run it from the project root after the server has created/applied the DB:
//
//	go run ./cmd/seed
//
// The command is idempotent: running it more than once does not create
// duplicate users, posts, or category links.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"forum/internal/db"

	"golang.org/x/crypto/bcrypt"
)

const defaultPassword = "password123"

type demoUser struct {
	ID        string
	Nickname  string
	Email     string
	Age       int
	Gender    string
	FirstName string
	LastName  string
}

type demoPost struct {
	ID         string
	UserID     string
	Title      string
	Content    string
	Categories []string
	CreatedAt  string
}

var demoUsers = []demoUser{
	{
		ID:        "demo-user-ada",
		Nickname:  "Ada",
		Email:     "ada@example.com",
		Age:       28,
		Gender:    "female",
		FirstName: "Ada",
		LastName:  "Lovelace",
	},
	{
		ID:        "demo-user-grace",
		Nickname:  "Grace",
		Email:     "grace@example.com",
		Age:       34,
		Gender:    "female",
		FirstName: "Grace",
		LastName:  "Hopper",
	},
	{
		ID:        "demo-user-lori",
		Nickname:  "Lori",
		Email:     "lori@example.com",
		Age:       25,
		Gender:    "other",
		FirstName: "Lori",
		LastName:  "Forum",
	},
}

var demoPosts = []demoPost{
	{
		ID:         "demo-post-welcome",
		UserID:     "demo-user-ada",
		Title:      "Welcome to the forum",
		Content:    "This is a small demo space for trying posts, comments, reactions, and chat.",
		Categories: []string{"General"},
		CreatedAt:  "2026-01-02 09:00:00",
	},
	{
		ID:         "demo-post-prompts",
		UserID:     "demo-user-grace",
		Title:      "Daily prompt: what are you building?",
		Content:    "Share one thing you are working on today and one thing that is currently blocking you.",
		Categories: []string{"Prompts"},
		CreatedAt:  "2026-01-02 10:00:00",
	},
	{
		ID:         "demo-post-tools",
		UserID:     "demo-user-lori",
		Title:      "Favorite tools for a real-time app",
		Content:    "SQLite, Go, vanilla JavaScript, and WebSockets make a surprisingly sturdy stack for a compact forum.",
		Categories: []string{"Tools & Apps"},
		CreatedAt:  "2026-01-02 11:00:00",
	},
	{
		ID:         "demo-post-jobs",
		UserID:     "demo-user-ada",
		Title:      "Junior-friendly backend checklist",
		Content:    "Auth, validation, migrations, indexes, logs, tests, and clear README steps are worth checking before review.",
		Categories: []string{"Jobs"},
		CreatedAt:  "2026-01-02 12:00:00",
	},
	{
		ID:         "demo-post-memes",
		UserID:     "demo-user-grace",
		Title:      "When WebSocket reconnect finally works",
		Content:    "That tiny green online dot feels much better after you have tested reconnects in three browser tabs.",
		Categories: []string{"Memes"},
		CreatedAt:  "2026-01-02 13:00:00",
	},
	{
		ID:         "demo-post-chat",
		UserID:     "demo-user-lori",
		Title:      "Chat test thread",
		Content:    "Log in as another demo user, open the sidebar, and send a message to check the real-time flow.",
		Categories: []string{"General", "Tools & Apps"},
		CreatedAt:  "2026-01-02 14:00:00",
	},
	{
		ID:         "demo-post-review",
		UserID:     "demo-user-ada",
		Title:      "Review notes",
		Content:    "Try editing your own post, reacting to someone else's post, and filtering by category.",
		Categories: []string{"General", "Prompts"},
		CreatedAt:  "2026-01-02 15:00:00",
	},
}

func main() {
	dbPath := os.Getenv("FORUM_DB")
	if dbPath == "" {
		dbPath = "forum.db"
	}

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	if err := seed(context.Background(), conn); err != nil {
		log.Fatalf("seed: %v", err)
	}

	log.Printf("seeded %d demo users and %d demo posts in %s", len(demoUsers), len(demoPosts), dbPath)
	log.Printf("demo password for Ada, Grace, and Lori: %s", defaultPassword)
}

func seed(ctx context.Context, conn *sql.DB) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, u := range demoUsers {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO users
				(id, nickname, email, password_hash, age, gender, first_name, last_name)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			u.ID, u.Nickname, u.Email, string(hash), u.Age, u.Gender, u.FirstName, u.LastName,
		); err != nil {
			return fmt.Errorf("insert user %s: %w", u.Nickname, err)
		}
		if err := verifyDemoUser(ctx, tx, u); err != nil {
			return err
		}
	}

	for _, p := range demoPosts {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO posts (id, user_id, title, content, created_at)
			VALUES (?, ?, ?, ?, ?)`,
			p.ID, p.UserID, p.Title, p.Content, p.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert post %s: %w", p.ID, err)
		}
		for _, name := range p.Categories {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO post_categories (post_id, category_id)
				SELECT ?, id FROM categories WHERE name = ?`,
				p.ID, name,
			); err != nil {
				return fmt.Errorf("link post %s to category %q: %w", p.ID, name, err)
			}
		}
	}

	return tx.Commit()
}

func verifyDemoUser(ctx context.Context, tx *sql.Tx, u demoUser) error {
	var id string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM users WHERE nickname = ? OR email = ?`,
		u.Nickname, u.Email,
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("verify user %s: %w", u.Nickname, err)
	}
	if id != u.ID {
		return fmt.Errorf("user %s or %s already exists with a different id", u.Nickname, u.Email)
	}
	return nil
}
