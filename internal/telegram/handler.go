package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Zaya-AS/tg-expense-bot/internal/expense"
)

type UserStore interface {
	EnsureUser(ctx context.Context, telegramUserID int64) error
}

type CategoryStore interface {
	CreateCategory(ctx context.Context, telegramUserID int64, name string) (bool, error)
}

type ExpenseStore interface {
	CreateExpense(
		ctx context.Context,
		telegramUserID int64,
		updateID int64,
		categoryName string,
		amountMinor int64,
		description string,
	) (bool, error)
}

type Handler struct {
	client     *Client
	users      UserStore
	categories CategoryStore
	expenses   ExpenseStore
}

func NewHandler(
	client *Client,
	users UserStore,
	categories CategoryStore,
	expenses ExpenseStore,
) *Handler {
	return &Handler{
		client:     client,
		users:      users,
		categories: categories,
		expenses:   expenses,
	}
}

func (h *Handler) Handle(ctx context.Context, update Update) error {
	if update.Message == nil {
		return nil
	}

	message := update.Message
	command, argument, _ := strings.Cut(strings.TrimSpace(message.Text), " ")

	switch command {
	case "/start":
		if message.From == nil {
			return nil
		}
		if err := h.users.EnsureUser(ctx, message.From.ID); err != nil {
			return err
		}
		return h.client.SendMessage(
			ctx,
			message.Chat.ID,
			"Привет! Я помогу записывать расходы.",
		)

	case "/category":
		if message.From == nil {
			return nil
		}

		name := strings.TrimSpace(argument)
		if name == "" {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Укажи название: /category Еда",
			)
		}

		if err := h.users.EnsureUser(ctx, message.From.ID); err != nil {
			return err
		}

		created, err := h.categories.CreateCategory(ctx, message.From.ID, name)
		if err != nil {
			return err
		}
		if !created {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Категория «"+name+"» уже есть.",
			)
		}
		return h.client.SendMessage(
			ctx, message.Chat.ID,
			"Категория «"+name+"» добавлена.",
		)

	case "/expense":
		if message.From == nil {
			return nil
		}

		parts := strings.SplitN(argument, "|", 3)
		if len(parts) < 2 {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Пример: /expense 250.50 | Еда | Обед",
			)
		}

		amountMinor, err := expense.ParseAmountMinor(parts[0])
		if err != nil {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Укажи положительную сумму с точностью до копеек, например 250.50.",
			)
		}

		categoryName := strings.TrimSpace(parts[1])
		if categoryName == "" {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Укажи категорию: /expense 250.50 | Еда | Обед",
			)
		}

		description := ""
		if len(parts) == 3 {
			description = strings.TrimSpace(parts[2])
		}

		if err := h.users.EnsureUser(ctx, message.From.ID); err != nil {
			return err
		}

		created, err := h.expenses.CreateExpense(
			ctx,
			message.From.ID,
			update.UpdateID,
			categoryName,
			amountMinor,
			description,
		)
		if errors.Is(err, expense.ErrCategoryNotFound) {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Категория «"+categoryName+"» не найдена. Сначала добавь её командой /category "+categoryName,
			)
		}
		if err != nil {
			return err
		}
		if !created {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Этот расход уже записан.",
			)
		}

		answer := fmt.Sprintf(
			"Записал %d.%02d RUB в категорию «%s».",
			amountMinor/100,
			amountMinor%100,
			categoryName,
		)
		return h.client.SendMessage(ctx, message.Chat.ID, answer)

	default:
		return nil
	}
}
