package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Zaya-AS/tg-expense-bot/internal/expense"
)

type exportExpenseStore struct {
	records []expense.Record
	userID  int64
}

func (*exportExpenseStore) CreateExpense(context.Context, int64, int64, string, int64, string) (bool, error) {
	panic("unexpected CreateExpense")
}
func (*exportExpenseStore) ListRecent(context.Context, int64, int) ([]expense.Record, error) {
	panic("unexpected ListRecent")
}
func (*exportExpenseStore) DeleteExpense(context.Context, int64, int64) (bool, error) {
	panic("unexpected DeleteExpense")
}
func (s *exportExpenseStore) ListForExport(_ context.Context, userID int64) ([]expense.Record, error) {
	s.userID = userID
	return s.records, nil
}

func TestExportCommandSendsExcelFile(t *testing.T) {
	sender := &recordingSender{}
	expenses := &exportExpenseStore{records: []expense.Record{{
		Category: "еда", AmountMinor: 25050, Currency: "RUB",
		Description: "Обед", SpentAt: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
	}}}
	handler := &Handler{client: sender, users: &testUserStore{zone: "UTC"}, expenses: expenses}
	err := handler.Handle(context.Background(), Update{Message: &Message{
		Chat: Chat{ID: 11, Type: "private"}, From: &User{ID: 11}, Text: "/export",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if expenses.userID != 11 || !strings.HasPrefix(sender.filename, "expenses-") || !strings.HasSuffix(sender.filename, ".xlsx") ||
		len(sender.document) < 4 || string(sender.document[:2]) != "PK" {
		t.Fatalf("export user = %d, filename = %q, size = %d", expenses.userID, sender.filename, len(sender.document))
	}
}

func TestExportCommandWithNoExpensesShowsHint(t *testing.T) {
	sender := &recordingSender{}
	handler := &Handler{client: sender, users: &testUserStore{zone: "UTC"}, expenses: &exportExpenseStore{}}
	err := handler.Handle(context.Background(), Update{Message: &Message{
		Chat: Chat{ID: 11, Type: "private"}, From: &User{ID: 11}, Text: "/export",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.document) != 0 || len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "Расходов") {
		t.Fatalf("document size = %d, messages = %v", len(sender.document), sender.messages)
	}
}
