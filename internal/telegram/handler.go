package telegram

import "context"

type UserStore interface {
	EnsureUser(ctx context.Context, telegramUserID int64) error
}

type Handler struct {
	client *Client
	users  UserStore
}

func NewHandler(client *Client, users UserStore) *Handler {
	return &Handler{
		client: client,
		users:  users,
	}
}

func (h *Handler) Handle(ctx context.Context, update Update) error {
	if update.Message == nil {
		return nil
	}

	switch update.Message.Text {
	case "/start":
		if update.Message.From == nil {
			return nil
		}
		if err := h.users.EnsureUser(ctx, update.Message.From.ID); err != nil {
			return err
		}

		return h.client.SendMessage(
			ctx,
			update.Message.Chat.ID,
			"Привет! Я помогу записывать расходы.",
		)
	default:
		return nil
	}
}
