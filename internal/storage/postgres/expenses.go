package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Zaya-AS/tg-expense-bot/internal/expense"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExpenseRepository struct {
	pool *pgxpool.Pool
}

func NewExpenseRepository(pool *pgxpool.Pool) *ExpenseRepository {
	return &ExpenseRepository{pool: pool}
}

func (r *ExpenseRepository) CreateExpense(
	ctx context.Context,
	telegramUserID int64,
	updateID int64,
	categoryName string,
	amountMinor int64,
	description string,
) (bool, error) {
	var userID, categoryID int64

	err := r.pool.QueryRow(ctx, `
		SELECT u.id, c.id
		FROM users AS u
		JOIN categories AS c ON c.user_id = u.id
		WHERE u.telegram_user_id = $1 AND c.name = $2
	`, telegramUserID, categoryName).Scan(&userID, &categoryID)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, expense.ErrCategoryNotFound
	}
	if err != nil {
		return false, fmt.Errorf("find expense category: %w", err)
	}

	result, err := r.pool.Exec(ctx, `
		INSERT INTO expenses (
			user_id, category_id, amount_minor, currency,
			description, spent_at, source_update_id
		)
		VALUES ($1, $2, $3, 'RUB', $4, NOW(), $5)
		ON CONFLICT (source_update_id) DO NOTHING
	`, userID, categoryID, amountMinor, description, updateID)
	if err != nil {
		return false, fmt.Errorf("create expense: %w", err)
	}

	return result.RowsAffected() == 1, nil
}
