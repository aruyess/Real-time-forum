package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// isFKViolation reports whether err is a SQLite foreign-key constraint failure.
// Callers translate it into a domain error (e.g. ErrPostNotFound) so handlers
// can return 404 instead of leaking the raw driver message.
func isFKViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY")
}

//go:embed schema.sql
var schema string

// Open opens an SQLite database at the given path, applies the embedded
// schema, and returns a ready-to-use *sql.DB.
func Open(path string) (*sql.DB, error) {
	dsn := path + "?_foreign_keys=on&_journal_mode=WAL"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// migrate applies post-schema fix-ups for databases created by older
// versions. Each step is gated so it stays idempotent across restarts.
func migrate(db *sql.DB) error {
	// messages.image_url was added later to support image attachments.
	var has int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name = 'image_url'`,
	).Scan(&has); err != nil {
		return err
	}
	if has == 0 {
		if _, err := db.Exec(`ALTER TABLE messages ADD COLUMN image_url TEXT`); err != nil {
			return err
		}
	}

	// Categories: rename old taxonomy to the current one (idempotent), then
	// seed the current set. Renaming preserves post_categories links because
	// the category id stays the same — only the name changes.
	renames := [][2]string{
		{"general", "General"},
		{"discussion", "Prompts"},
		{"tech", "Tools & Apps"},
		{"news", "Jobs"},
		{"help", "Memes"},
	}
	for _, r := range renames {
		if _, err := db.Exec(
			`UPDATE categories SET name = ? WHERE name = ?`, r[1], r[0],
		); err != nil {
			return err
		}
	}

	seeds := []string{"General", "Prompts", "Tools & Apps", "Jobs", "Memes"}
	for _, name := range seeds {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO categories (name) VALUES (?)`, name,
		); err != nil {
			return err
		}
	}

	return nil
}
