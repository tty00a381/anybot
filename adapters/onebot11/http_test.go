package onebot11

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/core/message"
)

func TestHTTPTransportCallRaw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/send_private_msg" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var params map[string]any
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Fatal(err)
		}
		if params["user_id"].(float64) != 42 {
			t.Fatalf("params = %#v", params)
		}
		_, _ = w.Write([]byte(`{"status":"ok","retcode":0,"data":{"message_id":7}}`))
	}))
	defer server.Close()

	adapter := HTTP(server.URL, "")
	client := adapter.Client()
	receipt, err := client.Send(context.Background(), core.ReplyTarget{UserID: "42"}, message.New(message.Text("hi")))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ID != "7" {
		t.Fatalf("receipt = %#v", receipt)
	}
}

func TestHTTPTransportAuthorization(t *testing.T) {
	transport := newHTTPTransport("", "127.0.0.1:0", newOptions([]Option{WithAccessToken("secret")}))
	req := httptest.NewRequest("POST", "/?access_token=secret", nil)
	if !transport.authorized(req) {
		t.Fatal("query token should be accepted")
	}
	req = httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	if !transport.authorized(req) {
		t.Fatal("bearer token should be accepted")
	}
	req = httptest.NewRequest("POST", "/", nil)
	if transport.authorized(req) {
		t.Fatal("missing token should be rejected")
	}
}

func TestHTTPTransportWebhookEvent(t *testing.T) {
	transport := newHTTPTransport("", "127.0.0.1:0", newOptions([]Option{WithAccessToken("secret")}))
	events := make(chan *Event, 1)
	server := httptest.NewServer(transport.handler(func(_ context.Context, event *Event) error {
		events <- event
		return nil
	}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/?access_token=secret", "application/json", bytes.NewBufferString(`{
		"post_type":"message",
		"message_type":"private",
		"self_id":10000,
		"user_id":42,
		"message":"ping"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || string(data) != `{"status":"ok"}` {
		t.Fatalf("status=%d body=%s", resp.StatusCode, data)
	}
	select {
	case event := <-events:
		if event.UserID != 42 || event.Message.Text() != "ping" {
			t.Fatalf("event = %#v", event)
		}
	default:
		t.Fatal("未收到事件")
	}
}

func TestHTTPTransportRejectsPublicWebhookWithoutToken(t *testing.T) {
	transport := newHTTPTransport("", "0.0.0.0:0", newOptions(nil))
	err := transport.Start(context.Background(), func(context.Context, *Event) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "requires access token") {
		t.Fatalf("err = %v", err)
	}
}

func TestHTTPTransportRejectsOversizedWebhook(t *testing.T) {
	transport := newHTTPTransport("", "127.0.0.1:0", newOptions([]Option{
		WithAccessToken("secret"),
		WithMaxEventBytes(8),
	}))
	server := httptest.NewServer(transport.handler(func(context.Context, *Event) error {
		t.Fatal("oversized event should not be dispatched")
		return nil
	}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/?access_token=secret", "application/json", bytes.NewBufferString(`{"post_type":"message"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
