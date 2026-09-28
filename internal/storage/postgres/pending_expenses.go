package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PendingExpenseRepository struct {
	pool *pgxpool.Pool
}

func NewPendingExpenseRepository(pool *pgxpool.Pool) *PendingExpenseRepository {
	return &PendingExpenseRepository{pool: pool}
}

// SaveDraft is idempotent for a Telegram update. A processed update cannot open
// a new draft when Telegram delivers it again.
func (r *PendingExpenseRepository) SaveDraft(ctx context.Context, telegramUserID, updateID, amountMinor int64, description string) (bool, error) {
	if _, err := r.pool.Exec(ctx, `DELETE FROM pending_expenses WHERE created_at <= NOW() - INTERVAL '1 day'`); err != nil {
		return false, fmt.Errorf("remove expired expense drafts: %w", err)
	}
	var savedID int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO pending_expenses (source_update_id, user_id, amount_minor, description)
		SELECT $2, u.id, $3, $4
		FROM users AS u
		WHERE u.telegram_user_id = $1
		  AND NOT EXISTS (SELECT 1 FROM expenses WHERE source_update_id = $2)
		ON CONFLICT (source_update_id) DO UPDATE
		SET source_update_id = EXCLUDED.source_update_id
		RETURNING source_update_id
	`, telegramUserID, updateID, amountMinor, description).Scan(&savedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("save expense draft: %w", err)
	}
	return true, nil
}

// CompleteDraft accepts only a category owned by the draft owner. Deleting the
// draft and inserting the expense happen in the same transaction.
func (r *PendingExpenseRepository) CompleteDraft(ctx context.Context, telegramUserID, updateID, categoryID int64) (bool, string, int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, "", 0, fmt.Errorf("begin expense draft completion: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID, amountMinor int64
	var description, categoryName string
	err = tx.QueryRow(ctx, `
		SELECT d.user_id, d.amount_minor, d.description, c.name
		FROM pending_expenses AS d
		JOIN users AS u ON u.id = d.user_id
		JOIN categories AS c ON c.user_id = d.user_id AND c.id = $3
		WHERE u.telegram_user_id = $1 AND d.source_update_id = $2
		  AND d.created_at > NOW() - INTERVAL '1 day'
		FOR UPDATE OF d, u
	`, telegramUserID, updateID, categoryID).Scan(&userID, &amountMinor, &description, &categoryName)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", 0, nil
	}
	if err != nil {
		return false, "", 0, fmt.Errorf("find expense draft: %w", err)
	}

	created, err := insertNumberedExpense(ctx, tx, userID, categoryID, amountMinor, description, updateID)
	if err != nil {
		return false, "", 0, fmt.Errorf("create expense from draft: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pending_expenses WHERE source_update_id = $1`, updateID); err != nil {
		return false, "", 0, fmt.Errorf("delete completed expense draft: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, "", 0, fmt.Errorf("commit expense draft completion: %w", err)
	}
	return created, categoryName, amountMinor, nil
}
