package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		pool: pool,
	}
}

func (r *UserRepository) EnsureUser(ctx context.Context, telegramUserID int64) error {
	_, err := r.pool.Exec(ctx, `
	INSERT INTO users (telegram_user_id)
	VALUES ($1)
	ON CONFLICT (telegram_user_id) DO NOTHING
	`, telegramUserID)

	if err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	return nil
}
