package telegram

import "context"

type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{
		client: client,
	}
}

func (h *Handler) Handle(ctx context.Context, update Update) error {
	if update.Message == nil {
		return nil
	}

	switch update.Message.Text {
	case "/start":
		return h.client.SendMessage(
			ctx,
			update.Message.Chat.ID,
			"Привет! Я помогу записывать расходы.",
		)
	default:
		return nil
	}
}
