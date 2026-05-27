package session_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"forum/internal/db"
	"forum/internal/models"
	"forum/internal/session"
)

// openDBWithUser opens a fresh SQLite database and inserts one user so the
// sessions FK to users(id) is satisfied. Returns ctx + the connection +
// the seeded user's ID.
func openDBWithUser(t *testing.T) (context.Context, *sql.DB, string) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ctx := context.Background()
	u := &models.User{
		ID: "u1", Nickname: "n", Email: "e@e.com", PasswordHash: "h",
		Age: 25, Gender: "female", FirstName: "F", LastName: "L",
	}
	if err := db.CreateUser(ctx, conn, u); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return ctx, conn, u.ID
}

// requestWithCookies builds a GET request carrying every cookie that the
// given response recorder accumulated. Mimics a follow-up call from the
// same browser.
func requestWithCookies(rec *httptest.ResponseRecorder) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestSessionRoundtrip(t *testing.T) {
	ctx, conn, userID := openDBWithUser(t)

	rec := httptest.NewRecorder()
	s, err := session.Create(ctx, conn, rec, userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.UserID != userID {
		t.Errorf("session.UserID = %q, want %q", s.UserID, userID)
	}

	// A request carrying the cookie resolves back to the same session.
	req := requestWithCookies(rec)
	got, err := session.FromRequest(ctx, conn, req)
	if err != nil {
		t.Fatalf("from request: %v", err)
	}
	if got.UserID != userID {
		t.Errorf("retrieved session points to %q, want %q", got.UserID, userID)
	}

	// After Destroy, the same cookie should no longer resolve.
	if err := session.Destroy(ctx, conn, httptest.NewRecorder(), req); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if _, err := session.FromRequest(ctx, conn, req); !errors.Is(err, session.ErrNoSession) {
		t.Errorf("after destroy: want ErrNoSession, got %v", err)
	}
}

// Verifies the "one open session per user" rule: creating a second session
// for the same user invalidates the first one.
func TestSessionCreateEvictsPrevious(t *testing.T) {
	ctx, conn, userID := openDBWithUser(t)

	rec1 := httptest.NewRecorder()
	s1, err := session.Create(ctx, conn, rec1, userID)
	if err != nil {
		t.Fatalf("create #1: %v", err)
	}

	rec2 := httptest.NewRecorder()
	s2, err := session.Create(ctx, conn, rec2, userID)
	if err != nil {
		t.Fatalf("create #2: %v", err)
	}
	if s1.Token == s2.Token {
		t.Fatal("tokens should differ between two Create calls")
	}

	// The first browser still holds the old cookie — it should no longer work.
	req1 := requestWithCookies(rec1)
	if _, err := session.FromRequest(ctx, conn, req1); !errors.Is(err, session.ErrNoSession) {
		t.Errorf("old session: want ErrNoSession, got %v", err)
	}

	// The new browser's cookie still resolves.
	req2 := requestWithCookies(rec2)
	if _, err := session.FromRequest(ctx, conn, req2); err != nil {
		t.Errorf("new session should still be valid, got %v", err)
	}
}
