package db

import (
	"context"
	"database/sql"

	"forum/internal/models"
)

// ListCategories returns all categories ordered by name.
func ListCategories(ctx context.Context, conn *sql.DB) ([]models.Category, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT id, name FROM categories ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
