package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Zaya-AS/tg-expense-bot/internal/expense"
)

const welcomeText = "Привет! Я помогу записывать расходы.\n\n" +
	"Напиши сумму и описание, например: 250.50 Обед. Затем выбери категорию кнопкой.\n" +
	"/category_all — посмотреть категории (также /category-all)\n" +
	"/category кафе — добавить категорию\n" +
	"/last — последние расходы; /delete 123 — удалить запись\n" +
	"/report — отчёт за месяц\n" +
	"/export — скачать расходы в Excel\n" +
	"/timezone Asia/Yekaterinburg — часовой пояс (изначально UTC)\n" +
	"/help — полная справка"

const guideText = "Как пользоваться:\n" +
	"250.50 Обед — отправь сумму и описание, затем нажми категорию. Описание можно не писать.\n" +
	"/category_all — все категории (также /category-all)\n" +
	"/category кафе — добавить категорию\n" +
	"/last — последние 10 расходов\n" +
	"/delete 123 — удалить расход №123 из /last\n" +
	"/report — отчёт за текущий месяц\n" +
	"/export — Excel с расходами по месяцам и общим итогом\n" +
	"/timezone Asia/Yekaterinburg — задать часовой пояс\n" +
	"/timezone — посмотреть часовой пояс\n" +
	"/start — приветствие и краткая инструкция"

func (h *Handler) sendCategories(ctx context.Context, chatID, telegramUserID int64) error {
	if err := h.users.EnsureUser(ctx, telegramUserID); err != nil {
		return err
	}
	names, err := h.categories.ListCategories(ctx, telegramUserID)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return h.client.SendMessage(ctx, chatID, "Категорий пока нет. Добавь первую: /category кафе")
	}
	return h.sendLongText(ctx, chatID, "Твои категории:\n• "+strings.Join(names, "\n• "))
}

func (h *Handler) beginExpense(ctx context.Context, chatID, telegramUserID, updateID int64, input string) error {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return h.client.SendMessage(ctx, chatID, "Напиши сумму и описание, например: 250.50 Обед")
	}
	amountMinor, err := expense.ParseAmountMinor(fields[0])
	if err != nil {
		if fields[0][0] >= '0' && fields[0][0] <= '9' {
			return h.client.SendMessage(ctx, chatID, "Сумма должна быть положительной, с точностью до копеек. Пример: 250.50 Обед")
		}
		return h.client.SendMessage(ctx, chatID, "Не понял сообщение. Чтобы записать расход, напиши: 250.50 Обед. Команды: /help")
	}
	description := strings.TrimSpace(strings.TrimPrefix(input, fields[0]))
	if err := h.users.EnsureUser(ctx, telegramUserID); err != nil {
		return err
	}
	options, err := h.categories.ListCategoryOptions(ctx, telegramUserID)
	if err != nil {
		return err
	}
	if len(options) == 0 {
		return h.client.SendMessage(ctx, chatID, "Категорий пока нет. Добавь первую: /category кафе")
	}
	saved, err := h.drafts.SaveDraft(ctx, telegramUserID, updateID, amountMinor, description)
	if err != nil {
		return err
	}
	if !saved {
		return h.client.SendMessage(ctx, chatID, "Этот расход уже записан.")
	}
	buttons := make([]Button, 0, len(options))
	for _, option := range options {
		buttons = append(buttons, Button{
			Text: option.Name,
			Data: fmt.Sprintf("expense:%d:%d", updateID, option.ID),
		})
	}
	shortDescription := description
	if utf8.RuneCountInString(shortDescription) > 100 {
		shortDescription = string([]rune(shortDescription)[:100]) + "…"
	}
	prompt := fmt.Sprintf("%d.%02d ₽", amountMinor/100, amountMinor%100)
	if shortDescription != "" {
		prompt += " — " + shortDescription
	}
	return h.client.SendMessageWithKeyboard(ctx, chatID, prompt+"\nВыбери категорию:", buttons)
}

func (h *Handler) handleCategorySelection(ctx context.Context, callback *CallbackQuery) error {
	if callback.Message == nil || callback.Message.Chat.Type != "private" {
		return h.client.AnswerCallbackQuery(ctx, callback.ID, "")
	}
	parts := strings.Split(callback.Data, ":")
	if len(parts) != 3 || parts[0] != "expense" {
		return h.client.AnswerCallbackQuery(ctx, callback.ID, "Неизвестное действие")
	}
	updateID, updateErr := strconv.ParseInt(parts[1], 10, 64)
	categoryID, categoryErr := strconv.ParseInt(parts[2], 10, 64)
	if updateErr != nil || categoryErr != nil || updateID <= 0 || categoryID <= 0 {
		return h.client.AnswerCallbackQuery(ctx, callback.ID, "Некорректная кнопка")
	}
	created, categoryName, amountMinor, err := h.drafts.CompleteDraft(ctx, callback.From.ID, updateID, categoryID)
	if err != nil {
		return err
	}
	if !created {
		return h.client.AnswerCallbackQuery(ctx, callback.ID, "Запись уже обработана или кнопка устарела")
	}
	if err := h.client.AnswerCallbackQuery(ctx, callback.ID, "Сохранено"); err != nil {
		return err
	}
	answer := fmt.Sprintf("Записал %d.%02d ₽ в категорию «%s».", amountMinor/100, amountMinor%100, categoryName)
	return h.client.SendMessage(ctx, callback.Message.Chat.ID, answer)
}
