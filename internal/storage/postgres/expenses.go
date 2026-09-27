package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Zaya-AS/tg-expense-bot/internal/category"
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
	categoryName = category.Normalize(categoryName)
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

func (r *ExpenseRepository) ListRecent(ctx context.Context, telegramUserID int64, limit int) ([]expense.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, c.name, e.amount_minor, e.currency, e.description, e.spent_at
		FROM expenses AS e
		JOIN users AS u ON u.id = e.user_id
		JOIN categories AS c ON c.id = e.category_id AND c.user_id = e.user_id
		WHERE u.telegram_user_id = $1
			AND e.deleted_at IS NULL
		ORDER BY e.spent_at DESC, e.id DESC
		LIMIT $2
	`, telegramUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent expenses: %w", err)
	}
	defer rows.Close()

	var records []expense.Record
	for rows.Next() {
		var item expense.Record
		if err := rows.Scan(&item.ID, &item.Category, &item.AmountMinor, &item.Currency, &item.Description, &item.SpentAt); err != nil {
			return nil, fmt.Errorf("scan recent expense: %w", err)
		}
		records = append(records, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read recent expenses: %w", err)
	}
	return records, nil
}

func (r *ExpenseRepository) DeleteExpense(ctx context.Context, telegramUserID, expenseID int64) (bool, error) {
	result, err := r.pool.Exec(ctx, `
		UPDATE expenses AS e
		SET deleted_at = NOW()
		FROM users AS u
		WHERE e.id = $1 AND e.user_id = u.id AND u.telegram_user_id = $2
			AND e.deleted_at IS NULL
	`, expenseID, telegramUserID)
	if err != nil {
		return false, fmt.Errorf("delete expense: %w", err)
	}
	return result.RowsAffected() == 1, nil
}
