package napcat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tty00a381/anybot"
	"github.com/tty00a381/anybot/message"
)

func TestHistoryAndRawFallback(t *testing.T) {
	client := &fakeClient{response: &anybot.ActionResponse{
		Status:  "ok",
		RetCode: 0,
		Data:    json.RawMessage(`{"messages":[{"message_id":"1","user_id":"42","group_id":"100","time":"10","raw_message":"hi"}]}`),
	}}
	api := New(client)
	messages, err := api.GetGroupMessageHistory(context.Background(), "100", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if client.action != "get_group_msg_history" || messages[0].MessageID != 1 || messages[0].UserID != 42 || messages[0].RawMessage != "hi" {
		t.Fatalf("action=%s messages=%#v", client.action, messages)
	}
	if _, err := api.CallRaw(context.Background(), "custom_action", map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	if client.action != "custom_action" {
		t.Fatalf("action = %s", client.action)
	}
}

func TestDownloadFile(t *testing.T) {
	client := &fakeClient{response: &anybot.ActionResponse{
		Status:  "ok",
		RetCode: 0,
		Data:    json.RawMessage(`{"file":"/tmp/a.png","size":"12"}`),
	}}
	api := New(client)
	info, err := api.DownloadFile(context.Background(), "https://example.com/a.png", 2, []string{"A=B"})
	if err != nil {
		t.Fatal(err)
	}
	if client.action != "download_file" || info.File == "" || info.Size != 12 {
		t.Fatalf("action=%s info=%#v", client.action, info)
	}
}

type fakeClient struct {
	action   string
	params   any
	response *anybot.ActionResponse
}

func (c *fakeClient) Call(_ context.Context, action string, params any, out any) error {
	c.action = action
	c.params = params
	return c.response.Decode(out)
}

func (c *fakeClient) CallRaw(_ context.Context, action string, params any) (*anybot.ActionResponse, error) {
	c.action = action
	c.params = params
	return c.response, nil
}

func (*fakeClient) Send(context.Context, anybot.ReplyTarget, message.Chain) (anybot.MessageReceipt, error) {
	return anybot.MessageReceipt{}, nil
}
