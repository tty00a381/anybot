package companionplugin

import (
	"context"
	"strings"
	"testing"

	absdk "github.com/tty00a381/anybot/sdk"
	"github.com/tty00a381/anybot/sdk/message"
)

func TestModuleRepliesAndStoresHistory(t *testing.T) {
	client := &recordClient{}
	app := absdk.NewApp(absdk.WithAdapter(recordAdapter{client: client}))
	if err := absdk.Install(app, Module); err != nil {
		t.Fatal(err)
	}
	if err := app.Dispatch(context.Background(), &absdk.Event{
		Protocol: "test",
		SelfID:   "bot",
		Type:     "message",
		UserID:   "user",
		GroupID:  "group",
		Text:     "/chat 今天服务器好安静",
	}); err != nil {
		t.Fatal(err)
	}
	if got := client.last.Text(); !strings.Contains(got, "我听见了：今天服务器好安静") {
		t.Fatalf("reply = %q", got)
	}
	ctx := absdk.NewContext(app, Module.Manifest())
	event := absdk.NewTestContext(app, &absdk.Event{
		Protocol: "test",
		Type:     "message",
		UserID:   "user",
		GroupID:  "group",
	})
	var history []chatMessage
	ok, err := ctx.UserSession(event).LoadJSON(context.Background(), "history", &history)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(history) != 2 || history[0].Content != "今天服务器好安静" {
		t.Fatalf("history ok=%v history=%#v", ok, history)
	}
}

func TestTrimForReplyDoesNotSplitChineseText(t *testing.T) {
	text := strings.Repeat("界", 81)
	got := trimForReply(text)
	if strings.Count(got, "界") != 80 || !strings.HasSuffix(got, "...") {
		t.Fatalf("trimmed = %q", got)
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
	last message.Chain
}

func (c *recordClient) Send(_ context.Context, _ absdk.ReplyTarget, chain message.Chain) (absdk.MessageReceipt, error) {
	c.last = chain
	return absdk.MessageReceipt{ID: "test"}, nil
}
