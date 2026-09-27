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
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	Chat Chat   `json:"chat"`
	From *User  `json:"from"`
	Text string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type User struct {
	ID int64 `json:"id"`
}

func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {

	params := struct {
		Offset  int64 `json:"offset"`
		Timeout int   `json:"timeout"`
	}{
		Offset:  offset,
		Timeout: 30,
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
