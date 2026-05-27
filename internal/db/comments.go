package db

import (
	"context"
	"database/sql"
	"errors"

	"forum/internal/models"
)

var ErrPostNotFound = errors.New("post not found")

// CreateComment inserts a new comment. A FK violation on post_id (post does
// not exist) is translated into ErrPostNotFound so handlers can return 404.
func CreateComment(ctx context.Context, conn *sql.DB, c *models.Comment) error {
	_, err := conn.ExecContext(ctx,
		`INSERT INTO comments (id, post_id, user_id, content) VALUES (?, ?, ?, ?)`,
		c.ID, c.PostID, c.UserID, c.Content,
	)
	if isFKViolation(err) {
		return ErrPostNotFound
	}
	return err
}

// UpdateComment replaces a comment's content. Returns sql.ErrNoRows when no
// row matches (deleted, or userID isn't the author).
func UpdateComment(ctx context.Context, conn *sql.DB, commentID, userID, content string) error {
	res, err := conn.ExecContext(ctx,
		`UPDATE comments SET content = ? WHERE id = ? AND user_id = ?`,
		content, commentID, userID,
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

// DeleteComment removes a comment (and via ON DELETE CASCADE, its reactions).
// Returns sql.ErrNoRows when userID isn't the author.
func DeleteComment(ctx context.Context, conn *sql.DB, commentID, userID string) error {
	res, err := conn.ExecContext(ctx,
		`DELETE FROM comments WHERE id = ? AND user_id = ?`,
		commentID, userID,
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

// ListCommentsForPost returns comments for a post, oldest first, joined with
// the author's nickname and reaction counts. viewerID may be empty
// (anonymous viewer) — my_reaction is then always 0.
func ListCommentsForPost(ctx context.Context, conn *sql.DB, postID, viewerID string, limit int) ([]models.Comment, error) {
	const q = `
		SELECT c.id, c.post_id, c.user_id, u.nickname, c.content, c.created_at,
		       (SELECT COUNT(*) FROM comment_reactions WHERE comment_id = c.id AND value =  1) AS likes,
		       (SELECT COUNT(*) FROM comment_reactions WHERE comment_id = c.id AND value = -1) AS dislikes,
		       COALESCE((SELECT value FROM comment_reactions WHERE comment_id = c.id AND user_id = ?), 0) AS my_reaction
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC
		LIMIT ?`
	rows, err := conn.QueryContext(ctx, q, viewerID, postID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Comment
	for rows.Next() {
		var c models.Comment
		if err := rows.Scan(
			&c.ID, &c.PostID, &c.UserID, &c.Author, &c.Content, &c.CreatedAt,
			&c.Likes, &c.Dislikes, &c.MyReaction,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
