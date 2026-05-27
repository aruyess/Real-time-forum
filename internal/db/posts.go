package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"forum/internal/models"
)

var ErrInvalidCategory = errors.New("one or more category IDs are invalid")

// CreatePost inserts a post and its category links in a single transaction.
// The caller must set p.ID, p.UserID, p.Title, p.Content; CreatedAt is filled
// from the DB default. categoryIDs may be empty.
func CreatePost(ctx context.Context, conn *sql.DB, p *models.Post, categoryIDs []int) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after Commit

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO posts (id, user_id, title, content) VALUES (?, ?, ?, ?)`,
		p.ID, p.UserID, p.Title, p.Content,
	); err != nil {
		return err
	}

	if len(categoryIDs) > 0 {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)`,
		)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, cid := range categoryIDs {
			if _, err := stmt.ExecContext(ctx, p.ID, cid); err != nil {
				if isFKViolation(err) {
					return ErrInvalidCategory
				}
				return err
			}
		}
	}

	return tx.Commit()
}

// PostListFilter combines all the optional filters /api/posts supports.
// Empty fields mean "no constraint"; ViewerID is used only to populate
// my_reaction (and may be empty for anonymous viewers).
//
// Categories has OR semantics: a post matches if it carries any of the
// listed category names. An empty slice means "don't filter by category".
type PostListFilter struct {
	Categories []string
	AuthorID   string
	LikedBy    string
	ViewerID   string
	Limit      int
}

// Column list shared by the single-row and list selects. Each item ends with
// likes, dislikes and my_reaction so the caller's Scan order stays stable.
const selectPostCols = `
	p.id, p.user_id, u.nickname, p.title, p.content, p.created_at,
	COALESCE(GROUP_CONCAT(DISTINCT c.name), '') AS cats,
	(SELECT COUNT(*) FROM post_reactions WHERE post_id = p.id AND value =  1) AS likes,
	(SELECT COUNT(*) FROM post_reactions WHERE post_id = p.id AND value = -1) AS dislikes,
	COALESCE((SELECT value FROM post_reactions WHERE post_id = p.id AND user_id = ?), 0) AS my_reaction`

const fromPostsJoin = `
	FROM posts p
	JOIN users u ON u.id = p.user_id
	LEFT JOIN post_categories pc ON pc.post_id = p.id
	LEFT JOIN categories      c  ON c.id      = pc.category_id`

// GetPost returns a single post by ID. viewerID may be empty (anonymous).
// Returns sql.ErrNoRows if the post does not exist.
func GetPost(ctx context.Context, conn *sql.DB, id, viewerID string) (*models.Post, error) {
	row := conn.QueryRowContext(ctx,
		`SELECT `+selectPostCols+fromPostsJoin+` WHERE p.id = ? GROUP BY p.id`,
		viewerID, id,
	)
	return scanPost(row.Scan)
}

// ListPosts returns posts newest-first matching the filter, joined with the
// author, categories and reaction counts.
func ListPosts(ctx context.Context, conn *sql.DB, f PostListFilter) ([]models.Post, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}

	conds := []string{}
	args := []any{f.ViewerID} // for my_reaction subquery
	if len(f.Categories) > 0 {
		// "?, ?, ..." with one placeholder per category.
		placeholders := strings.Repeat(",?", len(f.Categories))[1:]
		conds = append(conds, `EXISTS (
			SELECT 1 FROM post_categories pc2
			JOIN categories c2 ON c2.id = pc2.category_id
			WHERE pc2.post_id = p.id AND c2.name IN (`+placeholders+`)
		)`)
		for _, name := range f.Categories {
			args = append(args, name)
		}
	}
	if f.AuthorID != "" {
		conds = append(conds, `p.user_id = ?`)
		args = append(args, f.AuthorID)
	}
	if f.LikedBy != "" {
		conds = append(conds, `p.id IN (
			SELECT post_id FROM post_reactions WHERE user_id = ? AND value = 1
		)`)
		args = append(args, f.LikedBy)
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	q := `SELECT ` + selectPostCols + fromPostsJoin + where +
		` GROUP BY p.id ORDER BY p.created_at DESC LIMIT ?`
	args = append(args, f.Limit)

	rows, err := conn.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Post
	for rows.Next() {
		p, err := scanPost(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// UpdatePost replaces a post's title, content and category links. Returns
// sql.ErrNoRows if no row matches (post deleted, or userID doesn't match the
// author). Categories are replaced wholesale: old links are deleted and the
// new ones inserted in the same transaction. An invalid category surfaces as
// ErrInvalidCategory.
func UpdatePost(ctx context.Context, conn *sql.DB, postID, userID, title, content string, categoryIDs []int) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE posts SET title = ?, content = ? WHERE id = ? AND user_id = ?`,
		title, content, postID, userID,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM post_categories WHERE post_id = ?`, postID,
	); err != nil {
		return err
	}

	if len(categoryIDs) > 0 {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)`,
		)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, cid := range categoryIDs {
			if _, err := stmt.ExecContext(ctx, postID, cid); err != nil {
				if isFKViolation(err) {
					return ErrInvalidCategory
				}
				return err
			}
		}
	}

	return tx.Commit()
}

// DeletePost removes a post (and via ON DELETE CASCADE, its comments, category
// links and reactions). Returns sql.ErrNoRows if the userID isn't the author.
func DeletePost(ctx context.Context, conn *sql.DB, postID, userID string) error {
	res, err := conn.ExecContext(ctx,
		`DELETE FROM posts WHERE id = ? AND user_id = ?`,
		postID, userID,
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

// scanPost is a tiny adapter: both *sql.Row and *sql.Rows expose Scan with
// the same signature, so we accept the Scan func and reuse the same
// extraction logic for single-row and multi-row queries.
func scanPost(scan func(...any) error) (*models.Post, error) {
	var p models.Post
	var cats string
	err := scan(
		&p.ID, &p.UserID, &p.Author,
		&p.Title, &p.Content, &p.CreatedAt,
		&cats,
		&p.Likes, &p.Dislikes, &p.MyReaction,
	)
	if err != nil {
		return nil, err
	}
	if cats != "" {
		p.Categories = strings.Split(cats, ",")
	} else {
		p.Categories = []string{}
	}
	return &p, nil
}
