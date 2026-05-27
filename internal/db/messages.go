package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"forum/internal/models"
)

var ErrUserNotFound = errors.New("user not found")

// CreateMessage inserts a new message. On success the message's CreatedAt is
// populated from the DB. FK violations (unknown sender or receiver) are
// translated into ErrUserNotFound. ImageURL may be empty.
func CreateMessage(ctx context.Context, conn *sql.DB, m *models.Message) error {
	var img sql.NullString
	if m.ImageURL != "" {
		img = sql.NullString{String: m.ImageURL, Valid: true}
	}
	_, err := conn.ExecContext(ctx,
		`INSERT INTO messages (id, sender_id, receiver_id, content, image_url)
		 VALUES (?, ?, ?, ?, ?)`,
		m.ID, m.SenderID, m.ReceiverID, m.Content, img,
	)
	if isFKViolation(err) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	return conn.QueryRowContext(ctx,
		`SELECT created_at FROM messages WHERE id = ?`, m.ID,
	).Scan(&m.CreatedAt)
}

// MarkChatRead stamps "userID has read everything from peerID up to now"
// by upserting last_read_at = CURRENT_TIMESTAMP for that pair. Returns the
// timestamp that was written so the handler can echo it to the peer over
// WebSocket as a read receipt.
func MarkChatRead(ctx context.Context, conn *sql.DB, userID, peerID string) (time.Time, error) {
	now := time.Now().UTC()
	_, err := conn.ExecContext(ctx,
		`INSERT INTO chat_reads (user_id, peer_id, last_read_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(user_id, peer_id) DO UPDATE SET last_read_at = excluded.last_read_at`,
		userID, peerID, now,
	)
	return now, err
}

// PeerLastReadAt returns when peerID last opened the chat with viewerID
// (i.e. chat_reads.last_read_at where user_id=peerID, peer_id=viewerID).
// nil means "peer has never read this conversation"; the caller renders
// that as no read-receipt ticks on the viewer's outgoing messages.
func PeerLastReadAt(ctx context.Context, conn *sql.DB, viewerID, peerID string) (*time.Time, error) {
	var t time.Time
	err := conn.QueryRowContext(ctx,
		`SELECT last_read_at FROM chat_reads WHERE user_id = ? AND peer_id = ?`,
		peerID, viewerID,
	).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListUnreadPeers returns the set of peer IDs from which userID has
// messages newer than their stored last_read_at (or any messages at all
// for peers never opened). Used by GET /api/users to flag conversations
// with unread activity regardless of whether the user was online when the
// message arrived.
func ListUnreadPeers(ctx context.Context, conn *sql.DB, userID string) (map[string]bool, error) {
	const q = `
		SELECT DISTINCT m.sender_id
		FROM messages m
		LEFT JOIN chat_reads r
		  ON r.user_id = ? AND r.peer_id = m.sender_id
		WHERE m.receiver_id = ?
		  AND (r.last_read_at IS NULL OR m.created_at > r.last_read_at)`
	rows, err := conn.QueryContext(ctx, q, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ListMessages returns up to `limit` messages between selfID and otherID,
// newest first. If `before` is non-nil, returns only messages older than it
// (used for "load 10 more" scroll pagination).
func ListMessages(ctx context.Context, conn *sql.DB, selfID, otherID string, before *time.Time, limit int) ([]models.Message, error) {
	args := []any{selfID, otherID, otherID, selfID}
	query := `
		SELECT m.id, m.sender_id, m.receiver_id, u.nickname, m.content, m.image_url, m.created_at
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE ((m.sender_id = ? AND m.receiver_id = ?)
		    OR (m.sender_id = ? AND m.receiver_id = ?))`
	if before != nil {
		query += ` AND m.created_at < ?`
		args = append(args, *before)
	}
	query += ` ORDER BY m.created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Message
	for rows.Next() {
		var m models.Message
		var img sql.NullString
		if err := rows.Scan(
			&m.ID, &m.SenderID, &m.ReceiverID,
			&m.SenderNickname, &m.Content, &img, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		if img.Valid {
			m.ImageURL = img.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
