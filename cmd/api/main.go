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

	userRepo := postgres.NewUserRepository(pool)
	client := telegram.NewClient(cfg.TelegramBotToken)
	handler := telegram.NewHandler(client, userRepo)
	poller := telegram.NewPoller(client, handler)

	return poller.Run(ctx)
}
