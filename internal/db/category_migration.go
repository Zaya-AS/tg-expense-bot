package db

import (
	"context"
	"fmt"

	"github.com/Zaya-AS/tg-expense-bot/internal/category"
	"github.com/jackc/pgx/v5"
)

const categoryNormalizationVersion int64 = 3

type categoryKey struct {
	userID int64
	name   string
}

type categoryMove struct {
	userID int64
	fromID int64
	toID   int64
}

type categoryRename struct {
	id   int64
	name string
}

func normalizeCategories(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, `SELECT id, user_id, name FROM categories ORDER BY user_id, id`)
	if err != nil {
		return fmt.Errorf("read existing categories: %w", err)
	}
	keepers := make(map[categoryKey]int64)
	var moves []categoryMove
	var renames []categoryRename
	for rows.Next() {
		var id, userID int64
		var original string
		if err := rows.Scan(&id, &userID, &original); err != nil {
			rows.Close()
			return fmt.Errorf("scan existing category: %w", err)
		}
		name := category.Normalize(original)
		if name == "" {
			name = "другое"
		}
		key := categoryKey{userID: userID, name: name}
		if keeperID, exists := keepers[key]; exists {
			moves = append(moves, categoryMove{userID: userID, fromID: id, toID: keeperID})
			continue
		}
		keepers[key] = id
		if original != name {
			renames = append(renames, categoryRename{id: id, name: name})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read existing categories: %w", err)
	}
	rows.Close()

	for _, move := range moves {
		if _, err := tx.Exec(ctx, `
			UPDATE expenses SET category_id = $2
			WHERE user_id = $3 AND category_id = $1
		`, move.fromID, move.toID, move.userID); err != nil {
			return fmt.Errorf("move expenses from category %d: %w", move.fromID, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM categories WHERE id = $1 AND user_id = $2`, move.fromID, move.userID); err != nil {
			return fmt.Errorf("remove duplicate category %d: %w", move.fromID, err)
		}
	}
	for _, rename := range renames {
		if _, err := tx.Exec(ctx, `UPDATE categories SET name = $2 WHERE id = $1`, rename.id, rename.name); err != nil {
			return fmt.Errorf("normalize category %d: %w", rename.id, err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO categories (user_id, name)
		SELECT u.id, defaults.name
		FROM users AS u
		CROSS JOIN unnest($1::text[]) AS defaults(name)
		ON CONFLICT (user_id, name) DO NOTHING
	`, category.Defaults); err != nil {
		return fmt.Errorf("add default categories to existing users: %w", err)
	}
	return nil
}
