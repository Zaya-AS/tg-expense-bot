package telegram

import "context"

type botCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// RegisterCommands publishes the slash-command menu in private chats.
// Telegram commands may use underscores but cannot contain hyphens.
func (c *Client) RegisterCommands(ctx context.Context) error {
	params := struct {
		Commands []botCommand `json:"commands"`
		Scope    struct {
			Type string `json:"type"`
		} `json:"scope"`
	}{
		Commands: []botCommand{
			{Command: "start", Description: "Начать работу: краткая инструкция"},
			{Command: "help", Description: "Как пользоваться ботом"},
			{Command: "category", Description: "Добавить категорию: /category кафе"},
			{Command: "category_all", Description: "Показать все категории"},
			{Command: "last", Description: "Показать последние 10 расходов и их номера"},
			{Command: "delete", Description: "Удалить расход: /delete 123"},
			{Command: "report", Description: "Показать расходы за текущий месяц"},
			{Command: "export", Description: "Скачать все расходы по месяцам в Excel"},
			{Command: "timezone", Description: "Посмотреть или задать часовой пояс"},
		},
	}
	params.Scope.Type = "all_private_chats"
	return c.call(ctx, "setMyCommands", params, nil)
}
