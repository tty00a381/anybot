package hello

import (
	"context"
	"testing"

	absdk "github.com/tty00a381/anybot/sdk"
	"github.com/tty00a381/anybot/sdk/message"
)

func TestPluginRepliesToCommand(t *testing.T) {
	client := &recordClient{}
	app := absdk.NewApp(absdk.WithAdapter(recordAdapter{client: client}))
	if err := absdk.InstallDefault(app, Plugin); err != nil {
		t.Fatal(err)
	}
	if err := dispatch(app, "/hello"); err != nil {
		t.Fatal(err)
	}
	if got := client.last.Text(); got != "你好，我是 AnyBot 插件。" {
		t.Fatalf("reply = %q", got)
	}
}

func dispatch(app *absdk.App, text string) error {
	return app.Dispatch(context.Background(), &absdk.Event{
		Protocol: "test",
		SelfID:   "bot",
		Type:     "message",
		UserID:   "user",
		Text:     text,
	})
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
	last message.Chain
}

func (c *recordClient) Send(_ context.Context, _ absdk.ReplyTarget, chain message.Chain) (absdk.MessageReceipt, error) {
	c.last = chain.Clone()
	return absdk.MessageReceipt{ID: "test"}, nil
}
