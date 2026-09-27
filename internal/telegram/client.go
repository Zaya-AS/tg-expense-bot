package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	token string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 40 * time.Second},
	}
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Button struct {
	Text string
	Data string
}

type Message struct {
	Chat Chat   `json:"chat"`
	From *User  `json:"from"`
	Text string `json:"text"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type User struct {
	ID int64 `json:"id"`
}

func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {

	params := struct {
		Offset         int64    `json:"offset"`
		Timeout        int      `json:"timeout"`
		AllowedUpdates []string `json:"allowed_updates"`
	}{
		Offset:         offset,
		Timeout:        30,
		AllowedUpdates: []string{"message", "callback_query"},
	}

	var updates []Update

	if err := c.call(ctx, "getUpdates", params, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {

	params := struct {
		ChatID int64  `json:"chat_id"`
		Text   string `json:"text"`
	}{
		ChatID: chatID,
		Text:   text,
	}

	return c.call(ctx, "sendMessage", params, nil)
}

func (c *Client) SendMessageWithKeyboard(ctx context.Context, chatID int64, text string, buttons []Button) error {
	type inlineButton struct {
		Text         string `json:"text"`
		CallbackData string `json:"callback_data"`
	}
	var rows [][]inlineButton
	for i := 0; i < len(buttons); i += 2 {
		row := make([]inlineButton, 0, 2)
		for j := i; j < len(buttons) && j < i+2; j++ {
			row = append(row, inlineButton{Text: buttons[j].Text, CallbackData: buttons[j].Data})
		}
		rows = append(rows, row)
	}
	params := struct {
		ChatID      int64  `json:"chat_id"`
		Text        string `json:"text"`
		ReplyMarkup struct {
			InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
		} `json:"reply_markup"`
	}{ChatID: chatID, Text: text}
	params.ReplyMarkup.InlineKeyboard = rows
	return c.call(ctx, "sendMessage", params, nil)
}

func (c *Client) AnswerCallbackQuery(ctx context.Context, callbackID, text string) error {
	return c.call(ctx, "answerCallbackQuery", struct {
		CallbackQueryID string `json:"callback_query_id"`
		Text            string `json:"text,omitempty"`
	}{CallbackQueryID: callbackID, Text: text}, nil)
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("encode %s request: %w", method, err)
	}

	endpoint := "https://api.telegram.org/bot" + c.token + "/" + method
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create %s request", method)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("request %s: %w", method, err)
	}
	defer resp.Body.Close()

	var answer struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK || !answer.OK {
		return fmt.Errorf(
			"Telegram %s: HTTP %d: %s",
			method, resp.StatusCode, answer.Description,
		)
	}

	if result != nil {
		if err := json.Unmarshal(answer.Result, result); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil
}
