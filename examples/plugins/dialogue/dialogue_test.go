package dialogue

import (
	"context"
	"testing"

	absdk "github.com/tty00a381/anybot/sdk"
	"github.com/tty00a381/anybot/sdk/message"
)

func TestDialogueSavesProfile(t *testing.T) {
	client := &recordClient{}
	app := absdk.NewApp(absdk.WithAdapter(recordAdapter{client: client}))
	if err := absdk.InstallDefault(app, Plugin); err != nil {
		t.Fatal(err)
	}

	dispatch(t, app, "/whoami")
	if got := client.lastText(); got != "我还不认识你。先发送 /profile 建档。" {
		t.Fatalf("empty profile reply = %q", got)
	}
	dispatch(t, app, "/profile")
	if got := client.lastText(); got != "你想让我怎么称呼你？" {
		t.Fatalf("name prompt = %q", got)
	}
	dispatch(t, app, "小安")
	if got := client.lastText(); got != "你常在哪个城市出没？" {
		t.Fatalf("city prompt = %q", got)
	}
	dispatch(t, app, "杭州")
	if got := client.lastText(); got != "已记住：小安，杭州。" {
		t.Fatalf("saved reply = %q", got)
	}
	dispatch(t, app, "/whoami")
	if got := client.lastText(); got != "你是 小安，常在 杭州。" {
		t.Fatalf("summary reply = %q", got)
	}
}

func dispatch(t *testing.T, app *absdk.App, text string) {
	t.Helper()
	if err := app.Dispatch(context.Background(), &absdk.Event{
		Protocol: "test",
		SelfID:   "bot",
		Type:     "message",
		UserID:   "user",
		Text:     text,
	}); err != nil {
		t.Fatal(err)
	}
}

type recordAdapter struct {
	client *recordClient
}

func (a recordAdapter) Protocol() absdk.Protocol { return "test" }
func (a recordAdapter) Start(context.Context, absdk.EmitFunc) error {
	return nil
}
func (a recordAdapter) Client() absdk.ActionClient { return a.client }

type recordClient struct {
	sent []message.Chain
}

func (c *recordClient) Send(_ context.Context, _ absdk.ReplyTarget, chain message.Chain) (absdk.MessageReceipt, error) {
	c.sent = append(c.sent, chain.Clone())
	return absdk.MessageReceipt{ID: "test"}, nil
}

func (c *recordClient) lastText() string {
	if len(c.sent) == 0 {
		return ""
	}
	return c.sent[len(c.sent)-1].Text()
}
