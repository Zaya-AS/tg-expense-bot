package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRegisterCommandsUsesPrivateChatMenu(t *testing.T) {
	client := NewClient("test-token")
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/bottest-token/setMyCommands" {
			t.Errorf("method path = %q", req.URL.Path)
		}
		var params struct {
			Commands []botCommand `json:"commands"`
			Scope    struct {
				Type string `json:"type"`
			} `json:"scope"`
		}
		if err := json.NewDecoder(req.Body).Decode(&params); err != nil {
			t.Fatal(err)
		}
		if params.Scope.Type != "all_private_chats" {
			t.Errorf("scope = %q", params.Scope.Type)
		}
		seen := make(map[string]string)
		for _, command := range params.Commands {
			if command.Description == "" {
				t.Errorf("empty description for %q", command.Command)
			}
			seen[command.Command] = command.Description
		}
		for _, name := range []string{"start", "help", "category", "category_all", "last", "delete", "report", "export", "timezone"} {
			if seen[name] == "" {
				t.Errorf("missing %q command", name)
			}
		}
		if seen["category-all"] != "" {
			t.Error("Telegram command menu cannot register category-all")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)),
			Header:     make(http.Header),
		}, nil
	})}
	if err := client.RegisterCommands(context.Background()); err != nil {
		t.Fatal(err)
	}
}
