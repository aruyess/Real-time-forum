package session

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

const (
	CookieName = "session_token"
	TTL        = 7 * 24 * time.Hour
)

var ErrNoSession = errors.New("no session")

type Session struct {
	Token     string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Create issues a new session for userID, persists it, and sets the cookie on w.
//
// Per the spec ("each user may have only one opened session"), any existing
// session rows for this user are invalidated first — their tokens still live
// in other browsers, but the next request from those tabs will fail the
// session lookup and bounce the user back to the login screen.
func Create(ctx context.Context, conn *sql.DB, w http.ResponseWriter, userID string) (*Session, error) {
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ?`, userID,
	); err != nil {
		return nil, err
	}

	token, err := newToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(TTL)

	_, err = conn.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expires,
	)
	if err != nil {
		return nil, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return &Session{Token: token, UserID: userID, ExpiresAt: expires}, nil
}

// FromRequest reads the session cookie and looks it up. Returns ErrNoSession
// when the cookie is missing, the row is gone, or the session is expired.
func FromRequest(ctx context.Context, conn *sql.DB, r *http.Request) (*Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil, ErrNoSession
	}
	var s Session
	err = conn.QueryRowContext(ctx,
		`SELECT token, user_id, created_at, expires_at FROM sessions WHERE token = ?`,
		c.Value,
	).Scan(&s.Token, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoSession
		}
		return nil, err
	}
	if time.Now().After(s.ExpiresAt) {
		_, _ = conn.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, s.Token)
		return nil, ErrNoSession
	}
	return &s, nil
}

// Destroy removes the current session row (if any) and clears the cookie.
func Destroy(ctx context.Context, conn *sql.DB, w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(CookieName); err == nil {
		_, _ = conn.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
