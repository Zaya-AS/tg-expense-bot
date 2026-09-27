package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository struct {
	pool *pgxpool.Pool
}

func NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

func (r *CategoryRepository) CreateCategory(ctx context.Context, telegramUserID int64, name string) (bool, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO categories (user_id, name)
		SELECT id, $2 FROM users WHERE telegram_user_id = $1
		ON CONFLICT (user_id, name) DO NOTHING
		RETURNING id
	`, telegramUserID, name).Scan(&id)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create category: %w", err)
	}
	return true, nil
}

func (r *CategoryRepository) ListCategories(ctx context.Context, telegramUserID int64) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.name
		FROM categories AS c
		JOIN users AS u ON u.id = c.user_id
		WHERE u.telegram_user_id = $1
		ORDER BY c.name
	`, telegramUserID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}
	return names, nil
}
