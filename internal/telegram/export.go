package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/Zaya-AS/tg-expense-bot/internal/report"
)

func (h *Handler) exportExpenses(ctx context.Context, message *Message) error {
	if message.From == nil {
		return nil
	}
	userID := message.From.ID
	if err := h.users.EnsureUser(ctx, userID); err != nil {
		return err
	}
	zone, err := h.users.Timezone(ctx, userID)
	if err != nil {
		return err
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return fmt.Errorf("load export timezone: %w", err)
	}
	records, err := h.expenses.ListForExport(ctx, userID)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return h.client.SendMessage(ctx, message.Chat.ID, "Расходов для выгрузки пока нет.")
	}
	data, err := report.BuildExpensesWorkbook(records, location)
	if err != nil {
		return fmt.Errorf("build expense export: %w", err)
	}
	// Leave room for multipart fields within Telegram's document size limit.
	if len(data) > 49<<20 {
		return h.client.SendMessage(ctx, message.Chat.ID, "Файл с расходами слишком большой для отправки в Telegram.")
	}
	filename := "expenses-" + time.Now().In(location).Format("2006-01-02") + ".xlsx"
	return h.client.SendDocument(ctx, message.Chat.ID, filename, data)
}
