package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"forum/internal/models"
)

// newTestDB opens a fresh SQLite database in the test's temp dir. The schema
// is applied automatically by Open(). The DB is closed when the test ends.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func sampleUser(id, nick, email string) *models.User {
	return &models.User{
		ID:           id,
		Nickname:     nick,
		Email:        email,
		PasswordHash: "hashed",
		Age:          25,
		Gender:       "female",
		FirstName:    "F",
		LastName:     "L",
	}
}

func TestCreateUserRejectsDuplicates(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	if err := CreateUser(ctx, conn, sampleUser("u1", "alice", "a@a.com")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cases := []struct {
		name string
		u    *models.User
		want error
	}{
		{"duplicate nickname", sampleUser("u2", "alice", "other@a.com"), ErrNicknameTaken},
		{"duplicate email", sampleUser("u3", "bob", "a@a.com"), ErrEmailTaken},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := CreateUser(ctx, conn, c.u); !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestGetUserByLogin(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	if err := CreateUser(ctx, conn, sampleUser("u1", "alice", "a@a.com")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, ident := range []string{"alice", "a@a.com", "A@A.COM"} {
		t.Run(ident, func(t *testing.T) {
			got, err := GetUserByLogin(ctx, conn, ident)
			if err != nil {
				t.Fatalf("lookup: %v", err)
			}
			if got.ID != "u1" {
				t.Errorf("got id %q, want u1", got.ID)
			}
		})
	}

	if _, err := GetUserByLogin(ctx, conn, "nobody"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing user: want sql.ErrNoRows, got %v", err)
	}
}
