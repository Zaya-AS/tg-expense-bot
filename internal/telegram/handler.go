package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Zaya-AS/tg-expense-bot/internal/category"
	"github.com/Zaya-AS/tg-expense-bot/internal/expense"
	"github.com/Zaya-AS/tg-expense-bot/internal/report"
)

type MessageSender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendDocument(ctx context.Context, chatID int64, filename string, data []byte) error
	SendMessageWithKeyboard(ctx context.Context, chatID int64, text string, buttons []Button) error
	AnswerCallbackQuery(ctx context.Context, callbackID, text string) error
}

type ReportStore interface {
	CurrentMonth(
		ctx context.Context,
		telegramUserID int64,
	) (time.Time, []report.CategoryTotal, error)
}

type UserStore interface {
	EnsureUser(ctx context.Context, telegramUserID int64) error
	Timezone(ctx context.Context, telegramUserID int64) (string, error)
	SetTimezone(ctx context.Context, telegramUserID int64, timezone string) error
}

type CategoryStore interface {
	CreateCategory(ctx context.Context, telegramUserID int64, name string) (bool, error)
	ListCategories(ctx context.Context, telegramUserID int64) ([]string, error)
	ListCategoryOptions(ctx context.Context, telegramUserID int64) ([]category.Option, error)
}

type DraftStore interface {
	SaveDraft(ctx context.Context, telegramUserID, updateID, amountMinor int64, description string) (bool, error)
	CompleteDraft(ctx context.Context, telegramUserID, updateID, categoryID int64) (bool, string, int64, error)
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
	ListRecent(ctx context.Context, telegramUserID int64, limit int) ([]expense.Record, error)
	ListForExport(ctx context.Context, telegramUserID int64) ([]expense.Record, error)
	DeleteExpense(ctx context.Context, telegramUserID, expenseNumber int64) (bool, error)
}

type Handler struct {
	client     MessageSender
	users      UserStore
	categories CategoryStore
	expenses   ExpenseStore
	drafts     DraftStore
	reports    ReportStore
}

func NewHandler(
	client MessageSender,
	users UserStore,
	categories CategoryStore,
	expenses ExpenseStore,
	drafts DraftStore,
	reports ReportStore,
) *Handler {
	return &Handler{
		client:     client,
		users:      users,
		categories: categories,
		expenses:   expenses,
		drafts:     drafts,
		reports:    reports,
	}
}

func (h *Handler) Handle(ctx context.Context, update Update) error {
	if update.CallbackQuery != nil {
		return h.handleCategorySelection(ctx, update.CallbackQuery)
	}
	if update.Message == nil {
		return nil
	}
	if update.Message.Chat.Type != "private" {
		return nil
	}

	message := update.Message
	input := strings.TrimSpace(message.Text)
	if input == "" {
		return nil
	}
	if !strings.HasPrefix(input, "/") {
		if message.From == nil {
			return nil
		}
		return h.beginExpense(ctx, message.Chat.ID, message.From.ID, update.UpdateID, input)
	}
	command, argument, _ := strings.Cut(input, " ")

	switch command {
	case "/start":
		if message.From == nil {
			return nil
		}
		if err := h.users.EnsureUser(ctx, message.From.ID); err != nil {
			return err
		}
		return h.client.SendMessage(ctx, message.Chat.ID, welcomeText)

	case "/help":
		return h.client.SendMessage(ctx, message.Chat.ID, guideText)

	case "/timezone":
		if message.From == nil {
			return nil
		}
		if err := h.users.EnsureUser(ctx, message.From.ID); err != nil {
			return err
		}
		zone := strings.TrimSpace(argument)
		if zone == "" {
			current, err := h.users.Timezone(ctx, message.From.ID)
			if err != nil {
				return err
			}
			return h.client.SendMessage(ctx, message.Chat.ID, "Текущий часовой пояс: "+current+". Пример: /timezone Asia/Yekaterinburg")
		}
		if zone == "Local" {
			return h.client.SendMessage(ctx, message.Chat.ID, "Укажи часовой пояс вида Asia/Yekaterinburg или UTC.")
		}
		if _, err := time.LoadLocation(zone); err != nil {
			return h.client.SendMessage(ctx, message.Chat.ID, "Неизвестный часовой пояс. Пример: /timezone Asia/Yekaterinburg")
		}
		if err := h.users.SetTimezone(ctx, message.From.ID, zone); err != nil {
			return err
		}
		return h.client.SendMessage(ctx, message.Chat.ID, "Часовой пояс установлен: "+zone)

	case "/category":
		if message.From == nil {
			return nil
		}
		if strings.EqualFold(strings.TrimSpace(argument), "all") {
			return h.client.SendMessage(ctx, message.Chat.ID, "Для списка категорий используй /category_all или /category-all.")
		}

		name := category.Normalize(argument)
		if name == "" {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Укажи название: /category кафе",
			)
		}
		if utf8.RuneCountInString(name) > 100 || strings.ContainsAny(name, "|\r\n") {
			return h.client.SendMessage(ctx, message.Chat.ID, "Название категории должно быть короче 101 символа и не содержать | или перенос строки.")
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

	case "/category_all", "/category-all", "/categories":
		if message.From == nil {
			return nil
		}
		return h.sendCategories(ctx, message.Chat.ID, message.From.ID)

	case "/expense":
		if message.From == nil {
			return nil
		}
		if !strings.Contains(argument, "|") {
			return h.beginExpense(ctx, message.Chat.ID, message.From.ID, update.UpdateID, strings.TrimSpace(argument))
		}

		parts := strings.SplitN(argument, "|", 3)
		if len(parts) < 2 {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Пример: /expense 250.50 | еда | Обед",
			)
		}

		amountMinor, err := expense.ParseAmountMinor(parts[0])
		if err != nil {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Укажи положительную сумму с точностью до копеек, например 250.50.",
			)
		}

		categoryName := category.Normalize(parts[1])
		if categoryName == "" {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"Укажи категорию: /expense 250.50 | еда | Обед",
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

	case "/last":
		if message.From == nil {
			return nil
		}
		if err := h.users.EnsureUser(ctx, message.From.ID); err != nil {
			return err
		}
		records, err := h.expenses.ListRecent(ctx, message.From.ID, 10)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return h.client.SendMessage(ctx, message.Chat.ID, "Расходов пока нет.")
		}
		zone, err := h.users.Timezone(ctx, message.From.ID)
		if err != nil {
			return err
		}
		location, err := time.LoadLocation(zone)
		if err != nil {
			return fmt.Errorf("load user timezone: %w", err)
		}
		var answer strings.Builder
		answer.WriteString("Последние расходы:\n")
		for _, item := range records {
			description := strings.Join(strings.Fields(item.Description), " ")
			if utf8.RuneCountInString(description) > 80 {
				description = string([]rune(description)[:80]) + "…"
			}
			fmt.Fprintf(&answer, "#%d %s — %d.%02d %s (%s)", item.Number, item.Category, item.AmountMinor/100, item.AmountMinor%100, item.Currency, item.SpentAt.In(location).Format("02.01 15:04"))
			if description != "" {
				answer.WriteString(" — ")
				answer.WriteString(description)
			}
			answer.WriteByte('\n')
		}
		return h.sendLongText(ctx, message.Chat.ID, strings.TrimSuffix(answer.String(), "\n"))

	case "/delete":
		if message.From == nil {
			return nil
		}
		id, err := strconv.ParseInt(strings.TrimSpace(argument), 10, 64)
		if err != nil || id <= 0 {
			return h.client.SendMessage(ctx, message.Chat.ID, "Укажи номер расхода из /last. Пример: /delete 123")
		}
		deleted, err := h.expenses.DeleteExpense(ctx, message.From.ID, id)
		if err != nil {
			return err
		}
		if !deleted {
			return h.client.SendMessage(ctx, message.Chat.ID, "Расход не найден или уже удалён.")
		}
		return h.client.SendMessage(ctx, message.Chat.ID, fmt.Sprintf("Расход #%d удалён.", id))
	case "/report":
		if message.From == nil {
			return nil
		}
		if err := h.users.EnsureUser(ctx, message.From.ID); err != nil {
			return err
		}

		month, totals, err := h.reports.CurrentMonth(ctx, message.From.ID)
		if err != nil {
			return err
		}
		if len(totals) == 0 {
			return h.client.SendMessage(
				ctx, message.Chat.ID,
				"За "+month.Format("01.2006")+" расходов пока нет.",
			)
		}

		var answer strings.Builder
		fmt.Fprintf(&answer, "Расходы за %s:\n", month.Format("01.2006"))

		var totalMinor int64
		for _, item := range totals {
			fmt.Fprintf(
				&answer,
				"%s — %d.%02d ₽\n",
				item.Name,
				item.AmountMinor/100,
				item.AmountMinor%100,
			)
			totalMinor += item.AmountMinor
		}
		fmt.Fprintf(
			&answer,
			"Итого — %d.%02d ₽",
			totalMinor/100,
			totalMinor%100,
		)

		return h.sendLongText(ctx, message.Chat.ID, answer.String())
	case "/export":
		return h.exportExpenses(ctx, message)
	default:
		return h.client.SendMessage(ctx, message.Chat.ID, guideText)
	}
}

// Telegram accepts at most 4096 characters in one message. A byte limit below
// that value also keeps multi-byte UTF-8 text safely within the API limit.
func (h *Handler) sendLongText(ctx context.Context, chatID int64, message string) error {
	const maxBytes = 3500
	for len(message) > maxBytes {
		cut := strings.LastIndexByte(message[:maxBytes], '\n')
		if cut < 1 {
			cut = maxBytes
			for cut > 0 && !utf8.RuneStart(message[cut]) {
				cut--
			}
		}
		if err := h.client.SendMessage(ctx, chatID, message[:cut]); err != nil {
			return err
		}
		message = strings.TrimPrefix(message[cut:], "\n")
	}
	if message == "" {
		return nil
	}
	return h.client.SendMessage(ctx, chatID, message)
}
