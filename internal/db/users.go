package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"forum/internal/models"

	"github.com/mattn/go-sqlite3"
)

var (
	ErrNicknameTaken = errors.New("nickname is already taken")
	ErrEmailTaken    = errors.New("email is already registered")
)

// CreateUser inserts a new user. The caller is responsible for setting
// u.ID and u.PasswordHash beforehand. UNIQUE-constraint violations are
// translated into ErrNicknameTaken / ErrEmailTaken so handlers can react.
func CreateUser(ctx context.Context, conn *sql.DB, u *models.User) error {
	const q = `INSERT INTO users
		(id, nickname, email, password_hash, age, gender, first_name, last_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := conn.ExecContext(ctx, q,
		u.ID, u.Nickname, u.Email, u.PasswordHash,
		u.Age, u.Gender, u.FirstName, u.LastName,
	)
	if err == nil {
		return nil
	}

	var sErr sqlite3.Error
	if errors.As(err, &sErr) && sErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "users.nickname"):
			return ErrNicknameTaken
		case strings.Contains(msg, "users.email"):
			return ErrEmailTaken
		}
	}
	return err
}

const selectUserCols = `id, nickname, email, password_hash, age, gender, first_name, last_name, created_at`

// GetUserByID returns the user with the given ID. Returns sql.ErrNoRows if not found.
func GetUserByID(ctx context.Context, conn *sql.DB, id string) (*models.User, error) {
	row := conn.QueryRowContext(ctx, `SELECT `+selectUserCols+` FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// GetUserByLogin returns the user matching either nickname or email.
// The identifier is matched as-is against nickname and lowercased against email.
func GetUserByLogin(ctx context.Context, conn *sql.DB, identifier string) (*models.User, error) {
	row := conn.QueryRowContext(ctx,
		`SELECT `+selectUserCols+` FROM users WHERE nickname = ? OR email = ?`,
		identifier, strings.ToLower(identifier),
	)
	return scanUser(row)
}

// ProfileUpdate is the editable subset of a user's profile. Email and
// password are intentionally NOT here — email is immutable, password is
// changed via UpdateUserPassword.
type ProfileUpdate struct {
	Nickname  string
	FirstName string
	LastName  string
	Age       int
	Gender    string
}

// UpdateProfile applies a profile update for the given user. Returns
// sql.ErrNoRows if no user was matched (session points at a deleted id),
// or ErrNicknameTaken if the new nickname is in use by another account.
func UpdateProfile(ctx context.Context, conn *sql.DB, userID string, u ProfileUpdate) error {
	res, err := conn.ExecContext(ctx,
		`UPDATE users
		 SET nickname = ?, first_name = ?, last_name = ?, age = ?, gender = ?
		 WHERE id = ?`,
		u.Nickname, u.FirstName, u.LastName, u.Age, u.Gender, userID,
	)
	if err != nil {
		var sErr sqlite3.Error
		if errors.As(err, &sErr) && sErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			if strings.Contains(err.Error(), "users.nickname") {
				return ErrNicknameTaken
			}
		}
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateUserPassword replaces the stored password hash. The caller is
// responsible for verifying the current password and bcrypt-hashing the new
// one before calling this.
func UpdateUserPassword(ctx context.Context, conn *sql.DB, userID, newHash string) error {
	res, err := conn.ExecContext(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`,
		newHash, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ChatPartner is a sidebar-shaped view of another user: their identity plus
// the timestamp of the most recent message we've exchanged with them (NULL
// for users we've never messaged).
//
// LastMessageAt is kept as a string here because SQLite's MAX() over a
// DATETIME column drops the type affinity, so the driver can't scan it
// directly into time.Time. The handler parses it.
type ChatPartner struct {
	ID            string
	Nickname      string
	LastMessageAt sql.NullString
}

// ListChatPartners returns every user except selfID, ordered Discord-style:
//   - users with prior messages: by latest message DESC
//   - users with no messages:    by nickname (case-insensitive)
func ListChatPartners(ctx context.Context, conn *sql.DB, selfID string) ([]ChatPartner, error) {
	const q = `
		SELECT u.id, u.nickname, MAX(m.created_at) AS last_msg
		FROM users u
		LEFT JOIN messages m
		  ON (m.sender_id   = u.id AND m.receiver_id = ?)
		  OR (m.receiver_id = u.id AND m.sender_id   = ?)
		WHERE u.id != ?
		GROUP BY u.id
		ORDER BY
			CASE WHEN MAX(m.created_at) IS NULL THEN 1 ELSE 0 END,
			MAX(m.created_at) DESC,
			u.nickname COLLATE NOCASE`

	rows, err := conn.QueryContext(ctx, q, selfID, selfID, selfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChatPartner
	for rows.Next() {
		var p ChatPartner
		if err := rows.Scan(&p.ID, &p.Nickname, &p.LastMessageAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(
		&u.ID, &u.Nickname, &u.Email, &u.PasswordHash,
		&u.Age, &u.Gender, &u.FirstName, &u.LastName, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
