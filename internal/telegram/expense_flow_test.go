package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/Zaya-AS/tg-expense-bot/internal/category"
)

type flowCategories struct {
	options []category.Option
	created int
}

func (s *flowCategories) CreateCategory(context.Context, int64, string) (bool, error) {
	s.created++
	return true, nil
}
func (s *flowCategories) ListCategories(context.Context, int64) ([]string, error) {
	return []string{"еда", "кафе"}, nil
}
func (s *flowCategories) ListCategoryOptions(context.Context, int64) ([]category.Option, error) {
	return s.options, nil
}

type flowDrafts struct {
	userID      int64
	updateID    int64
	amountMinor int64
	description string
	completed   bool
}

func (s *flowDrafts) SaveDraft(_ context.Context, userID, updateID, amountMinor int64, description string) (bool, error) {
	s.userID, s.updateID, s.amountMinor, s.description = userID, updateID, amountMinor, description
	return true, nil
}
func (s *flowDrafts) CompleteDraft(_ context.Context, userID, updateID, categoryID int64) (bool, string, int64, error) {
	if s.completed || userID != s.userID || updateID != s.updateID || categoryID != 7 {
		return false, "", 0, nil
	}
	s.completed = true
	return true, "еда", s.amountMinor, nil
}

func TestBareExpenseUsesCategoryButton(t *testing.T) {
	sender := &recordingSender{}
	drafts := &flowDrafts{}
	handler := &Handler{
		client: sender, users: &testUserStore{zone: "UTC"},
		categories: &flowCategories{options: []category.Option{{ID: 7, Name: "еда"}}}, drafts: drafts,
	}
	ctx := context.Background()
	err := handler.Handle(ctx, Update{UpdateID: 42, Message: &Message{
		Chat: Chat{ID: 11, Type: "private"}, From: &User{ID: 11}, Text: "250,50 Обед",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if drafts.userID != 11 || drafts.updateID != 42 || drafts.amountMinor != 25050 || drafts.description != "Обед" {
		t.Fatalf("saved draft = %+v", drafts)
	}
	if len(sender.buttons) != 1 || sender.buttons[0].Text != "еда" || sender.buttons[0].Data != "expense:42:7" {
		t.Fatalf("buttons = %+v", sender.buttons)
	}
	callback := &CallbackQuery{ID: "callback-1", From: User{ID: 11},
		Message: &Message{Chat: Chat{ID: 11, Type: "private"}}, Data: sender.buttons[0].Data}
	if err := handler.Handle(ctx, Update{UpdateID: 43, CallbackQuery: callback}); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 2 || !strings.Contains(sender.messages[1], "250.50") || !drafts.completed {
		t.Fatalf("completion messages = %v, completed = %v", sender.messages, drafts.completed)
	}
	if err := handler.Handle(ctx, Update{UpdateID: 44, CallbackQuery: callback}); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 2 || len(sender.answers) != 2 || !strings.Contains(sender.answers[1], "уже обработана") {
		t.Fatalf("repeated selection messages = %v, answers = %v", sender.messages, sender.answers)
	}
}

func TestCategoryAllListsInsteadOfCreating(t *testing.T) {
	sender := &recordingSender{}
	categories := &flowCategories{}
	handler := &Handler{client: sender, users: &testUserStore{}, categories: categories}
	err := handler.Handle(context.Background(), Update{Message: &Message{
		Chat: Chat{ID: 11, Type: "private"}, From: &User{ID: 11}, Text: "/category all",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if categories.created != 0 || len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "кафе") {
		t.Fatalf("created = %d, messages = %v", categories.created, sender.messages)
	}
}

func TestStartExplainsUsage(t *testing.T) {
	sender := &recordingSender{}
	handler := &Handler{client: sender, users: &testUserStore{}}
	err := handler.Handle(context.Background(), Update{Message: &Message{
		Chat: Chat{ID: 11, Type: "private"}, From: &User{ID: 11}, Text: "/start",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "250.50 Обед") || !strings.Contains(sender.messages[0], "/category all") {
		t.Fatalf("welcome = %v", sender.messages)
	}
}
