package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

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
	if err := db.Migrate(ctx, pool, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	var versions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version IN (1, 2)`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 2 {
		t.Fatalf("applied versions = %d; want 2", versions)
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
	if err := users.SetTimezone(ctx, 11, "Asia/Yekaterinburg"); err != nil {
		t.Fatal(err)
	}
	if zone, err := users.Timezone(ctx, 11); err != nil || zone != "Asia/Yekaterinburg" {
		t.Fatalf("stored timezone = %q, %v", zone, err)
	}
	if created, err := categories.CreateCategory(ctx, 11, "Еда"); err != nil || !created {
		t.Fatalf("create category = %v, %v", created, err)
	}
	if _, err := expenses.CreateExpense(ctx, 22, 101, "Еда", 25050, "Обед"); !errors.Is(err, expense.ErrCategoryNotFound) {
		t.Fatalf("other user's category error = %v", err)
	}
	if created, err := expenses.CreateExpense(ctx, 11, 101, "Еда", 25050, "Обед"); err != nil || !created {
		t.Fatalf("create expense = %v, %v", created, err)
	}
	if created, err := expenses.CreateExpense(ctx, 11, 101, "Еда", 25050, "Обед"); err != nil || created {
		t.Fatalf("duplicate expense = %v, %v", created, err)
	}
	_, totals, err := reports.CurrentMonth(ctx, 11)
	if err != nil || len(totals) != 1 || totals[0].AmountMinor != 25050 {
		t.Fatalf("report before delete = %v, %v", totals, err)
	}
	items, err := expenses.ListRecent(ctx, 11, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("recent expenses = %v, %v", items, err)
	}
	if deleted, err := expenses.DeleteExpense(ctx, 22, items[0].ID); err != nil || deleted {
		t.Fatalf("delete other user's expense = %v, %v", deleted, err)
	}
	if deleted, err := expenses.DeleteExpense(ctx, 11, items[0].ID); err != nil || !deleted {
		t.Fatalf("delete own expense = %v, %v", deleted, err)
	}
	if created, err := expenses.CreateExpense(ctx, 11, 101, "Еда", 25050, "Обед"); err != nil || created {
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
}
