package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Zaya-AS/tg-expense-bot/internal/config"
	"github.com/Zaya-AS/tg-expense-bot/internal/db"
	"github.com/Zaya-AS/tg-expense-bot/internal/storage/postgres"
	"github.com/Zaya-AS/tg-expense-bot/internal/telegram"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	pool, err := db.NewPool(connectCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	log.Println("successfully connected to PostgreSQL")
	if err := db.Migrate(ctx, pool, "migrations"); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	log.Println("database migrations applied")

	userRepo := postgres.NewUserRepository(pool)
	categoryRepo := postgres.NewCategoryRepository(pool)
	expenseRepo := postgres.NewExpenseRepository(pool)
	pendingExpenseRepo := postgres.NewPendingExpenseRepository(pool)
	reportRepo := postgres.NewReportRepository(pool)

	client := telegram.NewClient(cfg.TelegramBotToken)
	if err := client.RegisterCommands(ctx); err != nil {
		return fmt.Errorf("register Telegram commands: %w", err)
	}
	handler := telegram.NewHandler(client, userRepo, categoryRepo, expenseRepo, pendingExpenseRepo, reportRepo)
	poller := telegram.NewPoller(client, handler)

	return poller.Run(ctx)
}
