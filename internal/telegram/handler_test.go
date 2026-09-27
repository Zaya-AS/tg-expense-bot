package telegram

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

type recordingSender struct {
	messages []string
	buttons  []Button
	answers  []string
}

func (s *recordingSender) SendMessage(_ context.Context, _ int64, message string) error {
	s.messages = append(s.messages, message)
	return nil
}

func (s *recordingSender) SendMessageWithKeyboard(_ context.Context, _ int64, message string, buttons []Button) error {
	s.messages = append(s.messages, message)
	s.buttons = append(s.buttons, buttons...)
	return nil
}

func (s *recordingSender) AnswerCallbackQuery(_ context.Context, _, answer string) error {
	s.answers = append(s.answers, answer)
	return nil
}

type testUserStore struct {
	zone string
}

func (s *testUserStore) EnsureUser(context.Context, int64) error { return nil }
func (s *testUserStore) Timezone(context.Context, int64) (string, error) {
	return s.zone, nil
}
func (s *testUserStore) SetTimezone(_ context.Context, _ int64, zone string) error {
	s.zone = zone
	return nil
}

func TestSendLongTextPreservesUTF8(t *testing.T) {
	sender := &recordingSender{}
	handler := &Handler{client: sender}
	message := strings.Repeat("界", 1500)
	if err := handler.sendLongText(context.Background(), 1, message); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) < 2 {
		t.Fatal("expected multiple Telegram messages")
	}
	for _, part := range sender.messages {
		if len(part) > 3500 || !utf8.ValidString(part) {
			t.Fatalf("invalid message part: %d bytes", len(part))
		}
	}
	if strings.Join(sender.messages, "") != message {
		t.Fatal("message changed while splitting")
	}
}

func TestHandlerIgnoresGroupMessages(t *testing.T) {
	sender := &recordingSender{}
	handler := &Handler{client: sender}
	err := handler.Handle(context.Background(), Update{
		Message: &Message{
			Chat: Chat{ID: -1, Type: "supergroup"},
			From: &User{ID: 11},
			Text: "/report",
		},
	})
	if err != nil || len(sender.messages) != 0 {
		t.Fatalf("group message result = %v, sent = %v", err, sender.messages)
	}
}

func TestTimezoneCommand(t *testing.T) {
	sender := &recordingSender{}
	users := &testUserStore{zone: "UTC"}
	handler := &Handler{client: sender, users: users}
	update := Update{Message: &Message{
		Chat: Chat{ID: 1, Type: "private"},
		From: &User{ID: 11},
		Text: "/timezone Asia/Yekaterinburg",
	}}
	if err := handler.Handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if users.zone != "Asia/Yekaterinburg" || len(sender.messages) != 1 {
		t.Fatalf("timezone result = %q, messages = %v", users.zone, sender.messages)
	}
	update.Message.Text = "/timezone Not/A_Zone"
	if err := handler.Handle(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if users.zone != "Asia/Yekaterinburg" {
		t.Fatal("invalid timezone changed stored value")
	}
}
