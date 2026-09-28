package telegram

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSendDocumentUploadsWorkbook(t *testing.T) {
	client := NewClient("test-token")
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/bottest-token/sendDocument" {
			t.Errorf("method path = %q", req.URL.Path)
		}
		if err := req.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if got := req.FormValue("chat_id"); got != "123" {
			t.Errorf("chat_id = %q", got)
		}
		file, header, err := req.FormFile("document")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if header.Filename != "expenses.xlsx" || !bytes.Equal(data, []byte("PK-test")) {
			t.Errorf("uploaded %q: %q", header.Filename, data)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{}}`)),
			Header:     make(http.Header),
		}, nil
	})}
	if err := client.SendDocument(context.Background(), 123, "expenses.xlsx", []byte("PK-test")); err != nil {
		t.Fatal(err)
	}
}
