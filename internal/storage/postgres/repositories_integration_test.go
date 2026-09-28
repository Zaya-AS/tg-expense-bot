package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Zaya-AS/tg-expense-bot/internal/category"
	"github.com/Zaya-AS/tg-expense-bot/internal/db"
	"github.com/Zaya-AS/tg-expense-bot/internal/expense"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	admin, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("tg_expense_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close(ctx)
		t.Fatal(err)
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		admin.Close(cleanupCtx)
	})

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+schema)
		return err
	}
	pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestMigrateExistingSchema(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	initialSQL, err := os.ReadFile("../../../migrations/000001_create_core_tables.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(initialSQL), pgx.QueryExecModeSimpleProtocol); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (telegram_user_id) VALUES (11), (22)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO categories (user_id, name)
		SELECT id, 'Еда' FROM users WHERE telegram_user_id = 11
		UNION ALL
		SELECT id, 'еда' FROM users WHERE telegram_user_id = 11
		UNION ALL
		SELECT id, 'ЕДА' FROM users WHERE telegram_user_id = 22
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO expenses (user_id, category_id, amount_minor, currency, spent_at, source_update_id)
		SELECT user_id, id, 100, 'RUB', NOW(), id FROM categories
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	var versions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version IN (1, 2, 3, 4, 5)`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 5 {
		t.Fatalf("applied versions = %d; want 5", versions)
	}
	var normalizedCount, expenseCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM categories AS c
		JOIN users AS u ON u.id = c.user_id
		WHERE u.telegram_user_id = 11 AND c.name = 'еда'
	`).Scan(&normalizedCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM expenses AS e
		JOIN users AS u ON u.id = e.user_id
		JOIN categories AS c ON c.id = e.category_id
		WHERE u.telegram_user_id = 11 AND c.name = 'еда'
	`).Scan(&expenseCount); err != nil {
		t.Fatal(err)
	}
	if normalizedCount != 1 || expenseCount != 2 {
		t.Fatalf("normalized categories = %d, preserved expenses = %d; want 1 and 2", normalizedCount, expenseCount)
	}
	for _, tc := range []struct {
		telegramUserID int64
		wantNumbers    int
	}{{11, 2}, {22, 1}} {
		var count int
		if err := pool.QueryRow(ctx, `
			SELECT count(DISTINCT e.user_number)
			FROM expenses AS e JOIN users AS u ON u.id = e.user_id
			WHERE u.telegram_user_id = $1 AND e.user_number BETWEEN 1 AND $2
		`, tc.telegramUserID, tc.wantNumbers).Scan(&count); err != nil || count != tc.wantNumbers {
			t.Fatalf("migrated numbers for user %d = %d, %v; want %d", tc.telegramUserID, count, err, tc.wantNumbers)
		}
	}
	var categoryCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM categories`).Scan(&categoryCount); err != nil {
		t.Fatal(err)
	}
	if categoryCount != 18 {
		t.Fatalf("existing users have %d categories; want 18 defaults", categoryCount)
	}
	rows, err := pool.Query(ctx, `SELECT name FROM categories`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if category.Normalize(name) != name {
			rows.Close()
			t.Fatalf("category %q was not normalized", name)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
}

func TestPendingExpenseSelectionIsOwnedAndIdempotent(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	if err := db.Migrate(ctx, pool, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	users := NewUserRepository(pool)
	for _, userID := range []int64{11, 22} {
		if err := users.EnsureUser(ctx, userID); err != nil {
			t.Fatal(err)
		}
	}
	categories := NewCategoryRepository(pool)
	options, err := categories.ListCategoryOptions(ctx, 11)
	if err != nil || len(options) == 0 {
		t.Fatalf("options = %v, %v", options, err)
	}
	otherOptions, err := categories.ListCategoryOptions(ctx, 22)
	if err != nil || len(otherOptions) == 0 {
		t.Fatalf("other options = %v, %v", otherOptions, err)
	}
	drafts := NewPendingExpenseRepository(pool)
	if saved, err := drafts.SaveDraft(ctx, 11, 100, 25050, "Обед"); err != nil || !saved {
		t.Fatalf("save draft = %v, %v", saved, err)
	}
	if created, _, _, err := drafts.CompleteDraft(ctx, 22, 100, options[0].ID); err != nil || created {
		t.Fatalf("other user completed draft = %v, %v", created, err)
	}
	if created, _, _, err := drafts.CompleteDraft(ctx, 11, 100, otherOptions[0].ID); err != nil || created {
		t.Fatalf("other user's category completed draft = %v, %v", created, err)
	}
	created, name, amount, err := drafts.CompleteDraft(ctx, 11, 100, options[0].ID)
	if err != nil || !created || name != options[0].Name || amount != 25050 {
		t.Fatalf("complete draft = %v, %q, %d, %v", created, name, amount, err)
	}
	items, err := NewExpenseRepository(pool).ListRecent(ctx, 11, 10)
	if err != nil || len(items) != 1 || items[0].Number != 1 {
		t.Fatalf("first expense from draft = %v, %v", items, err)
	}
	if created, _, _, err := drafts.CompleteDraft(ctx, 11, 100, options[0].ID); err != nil || created {
		t.Fatalf("repeated callback = %v, %v", created, err)
	}
	if saved, err := drafts.SaveDraft(ctx, 11, 100, 25050, "Обед"); err != nil || saved {
		t.Fatalf("replayed update = %v, %v", saved, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM expenses WHERE source_update_id = 100 AND amount_minor = 25050 AND description = 'Обед'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("saved expenses = %d, %v", count, err)
	}
}

func TestExpensesAreOwnedAndDeletionIsIdempotent(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	if err := db.Migrate(ctx, pool, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	users := NewUserRepository(pool)
	categories := NewCategoryRepository(pool)
	expenses := NewExpenseRepository(pool)
	reports := NewReportRepository(pool)
	for _, telegramID := range []int64{11, 22} {
		if err := users.EnsureUser(ctx, telegramID); err != nil {
			t.Fatal(err)
		}
	}
	names, err := categories.ListCategories(ctx, 11)
	if err != nil || len(names) != 9 {
		t.Fatalf("new user's categories = %v, %v; want 9 defaults", names, err)
	}
	if err := users.SetTimezone(ctx, 11, "Asia/Yekaterinburg"); err != nil {
		t.Fatal(err)
	}
	if zone, err := users.Timezone(ctx, 11); err != nil || zone != "Asia/Yekaterinburg" {
		t.Fatalf("stored timezone = %q, %v", zone, err)
	}
	if created, err := categories.CreateCategory(ctx, 11, "КАФЕ"); err != nil || !created {
		t.Fatalf("create category = %v, %v", created, err)
	}
	if created, err := categories.CreateCategory(ctx, 11, "кафе"); err != nil || created {
		t.Fatalf("duplicate category after normalization = %v, %v", created, err)
	}
	if _, err := expenses.CreateExpense(ctx, 22, 101, "КАФЕ", 25050, "Обед"); !errors.Is(err, expense.ErrCategoryNotFound) {
		t.Fatalf("other user's category error = %v", err)
	}
	if created, err := expenses.CreateExpense(ctx, 11, 101, "КАФЕ", 25050, "Обед"); err != nil || !created {
		t.Fatalf("create expense = %v, %v", created, err)
	}
	if created, err := expenses.CreateExpense(ctx, 11, 101, "кафе", 25050, "Обед"); err != nil || created {
		t.Fatalf("duplicate expense = %v, %v", created, err)
	}
	_, totals, err := reports.CurrentMonth(ctx, 11)
	if err != nil || len(totals) != 1 || totals[0].AmountMinor != 25050 {
		t.Fatalf("report before delete = %v, %v", totals, err)
	}
	items, err := expenses.ListRecent(ctx, 11, 10)
	if err != nil || len(items) != 1 || items[0].Number != 1 {
		t.Fatalf("recent expenses = %v, %v", items, err)
	}
	if deleted, err := expenses.DeleteExpense(ctx, 22, items[0].Number); err != nil || deleted {
		t.Fatalf("delete other user's expense = %v, %v", deleted, err)
	}
	if created, err := expenses.CreateExpense(ctx, 22, 102, "еда", 10000, "Ужин"); err != nil || !created {
		t.Fatalf("create other user's expense = %v, %v", created, err)
	}
	otherItems, err := expenses.ListRecent(ctx, 22, 10)
	if err != nil || len(otherItems) != 1 || otherItems[0].Number != 1 {
		t.Fatalf("other user's first expense = %v, %v", otherItems, err)
	}
	if deleted, err := expenses.DeleteExpense(ctx, 11, items[0].Number); err != nil || !deleted {
		t.Fatalf("delete own expense = %v, %v", deleted, err)
	}
	otherItems, err = expenses.ListRecent(ctx, 22, 10)
	if err != nil || len(otherItems) != 1 {
		t.Fatalf("other user's expense after deletion = %v, %v", otherItems, err)
	}
	if created, err := expenses.CreateExpense(ctx, 11, 101, "кафе", 25050, "Обед"); err != nil || created {
		t.Fatalf("replayed deleted expense = %v, %v", created, err)
	}
	items, err = expenses.ListRecent(ctx, 11, 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("recent expenses after delete = %v, %v", items, err)
	}
	_, totals, err = reports.CurrentMonth(ctx, 11)
	if err != nil || len(totals) != 0 {
		t.Fatalf("report after delete = %v, %v", totals, err)
	}
	if created, err := expenses.CreateExpense(ctx, 11, 103, "кафе", 30000, "Завтрак"); err != nil || !created {
		t.Fatalf("create next expense = %v, %v", created, err)
	}
	items, err = expenses.ListRecent(ctx, 11, 10)
	if err != nil || len(items) != 1 || items[0].Number != 2 {
		t.Fatalf("number after deletion = %v, %v; want 2", items, err)
	}
}
