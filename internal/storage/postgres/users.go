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

func (r *UserRepository) Timezone(ctx context.Context, telegramUserID int64) (string, error) {
	var timezone string
	err := r.pool.QueryRow(ctx, `SELECT timezone FROM users WHERE telegram_user_id = $1`, telegramUserID).Scan(&timezone)
	if err != nil {
		return "", fmt.Errorf("get user timezone: %w", err)
	}
	return timezone, nil
}

func (r *UserRepository) SetTimezone(ctx context.Context, telegramUserID int64, timezone string) error {
	result, err := r.pool.Exec(ctx, `UPDATE users SET timezone = $2 WHERE telegram_user_id = $1`, telegramUserID, timezone)
	if err != nil {
		return fmt.Errorf("set user timezone: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("set user timezone: user %d not found", telegramUserID)
	}
	return nil
}
