package db

import (
	"context"
	"database/sql"
	"errors"
)

// ReactionCounts is the JSON-shaped reply for any reaction toggle:
// total likes/dislikes on the target plus what THIS user's current
// reaction is (0 = none).
type ReactionCounts struct {
	Likes      int `json:"likes"`
	Dislikes   int `json:"dislikes"`
	MyReaction int `json:"myReaction"`
}

// SetPostReaction stores (or clears, when value == 0) the given user's
// reaction to a post and returns the fresh counts. Unknown post IDs surface
// as ErrPostNotFound via the FK violation on the underlying INSERT.
func SetPostReaction(ctx context.Context, conn *sql.DB, postID, userID string, value int) (ReactionCounts, error) {
	return setReaction(ctx, conn, "post_reactions", "post_id", postID, userID, value)
}

// SetCommentReaction is the same but for comments. Unknown comment IDs surface
// as ErrCommentNotFound.
func SetCommentReaction(ctx context.Context, conn *sql.DB, commentID, userID string, value int) (ReactionCounts, error) {
	return setReaction(ctx, conn, "comment_reactions", "comment_id", commentID, userID, value)
}

var ErrCommentNotFound = errors.New("comment not found")

// setReaction is the shared implementation. Tables and column names are
// passed in, so the SQL is parameterised by table identifier — only used
// internally with hard-coded literals, never with user input.
func setReaction(
	ctx context.Context, conn *sql.DB,
	table, targetCol, targetID, userID string, value int,
) (ReactionCounts, error) {
	var out ReactionCounts

	if value == 0 {
		if _, err := conn.ExecContext(ctx,
			`DELETE FROM `+table+` WHERE user_id = ? AND `+targetCol+` = ?`,
			userID, targetID,
		); err != nil {
			return out, err
		}
	} else {
		_, err := conn.ExecContext(ctx,
			`INSERT INTO `+table+` (user_id, `+targetCol+`, value) VALUES (?, ?, ?)
			 ON CONFLICT(user_id, `+targetCol+`) DO UPDATE SET value = excluded.value`,
			userID, targetID, value,
		)
		if isFKViolation(err) {
			if targetCol == "post_id" {
				return out, ErrPostNotFound
			}
			return out, ErrCommentNotFound
		}
		if err != nil {
			return out, err
		}
	}

	row := conn.QueryRowContext(ctx,
		`SELECT
			(SELECT COUNT(*) FROM `+table+` WHERE `+targetCol+` = ? AND value =  1),
			(SELECT COUNT(*) FROM `+table+` WHERE `+targetCol+` = ? AND value = -1),
			COALESCE((SELECT value FROM `+table+` WHERE `+targetCol+` = ? AND user_id = ?), 0)`,
		targetID, targetID, targetID, userID,
	)
	if err := row.Scan(&out.Likes, &out.Dislikes, &out.MyReaction); err != nil {
		return out, err
	}
	return out, nil
}
