package groupmemo

import (
	"context"
	"testing"

	absdk "github.com/tty00a381/anybot/sdk"
	"github.com/tty00a381/anybot/sdk/message"
	"gopkg.in/yaml.v3"
)

func TestGroupMemoStoresPerGroup(t *testing.T) {
	client := &recordClient{}
	app := absdk.NewApp(absdk.WithAdapter(recordAdapter{client: client}), absdk.WithSuperUsers("root"))
	if err := absdk.InstallDefault(app, Plugin); err != nil {
		t.Fatal(err)
	}

	if err := dispatch(app, "root", "100", "/remember 晚上八点开黑"); err != nil {
		t.Fatal(err)
	}
	if got := client.lastText(); got != "群便签已更新。" {
		t.Fatalf("set reply = %q", got)
	}
	if err := dispatch(app, "alice", "100", "/memo"); err != nil {
		t.Fatal(err)
	}
	if got := client.lastText(); got != "晚上八点开黑" {
		t.Fatalf("memo reply = %q", got)
	}
	if err := dispatch(app, "alice", "200", "/memo"); err != nil {
		t.Fatal(err)
	}
	if got := client.lastText(); got != "这个群还没有便签。" {
		t.Fatalf("other group reply = %q", got)
	}
}

func TestGroupMemoAllowedGroups(t *testing.T) {
	client := &recordClient{}
	app := absdk.NewApp(absdk.WithAdapter(recordAdapter{client: client}), absdk.WithSuperUsers("root"))
	plugin := buildPlugin(t, `view_command: memo
set_command: remember
allowed_groups:
  - "100"
`)
	if err := absdk.Install(app, plugin); err != nil {
		t.Fatal(err)
	}
	if err := dispatch(app, "root", "200", "/remember 不应该写入"); err != nil {
		t.Fatal(err)
	}
	if got := client.lastText(); got != "" {
		t.Fatalf("disallowed group should not reply, got %q", got)
	}
	if err := dispatch(app, "root", "100", "/remember 可以写入"); err != nil {
		t.Fatal(err)
	}
	if got := client.lastText(); got != "群便签已更新。" {
		t.Fatalf("allowed group reply = %q", got)
	}
}

func buildPlugin(t *testing.T, config string) absdk.Plugin {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(config), &node); err != nil {
		t.Fatal(err)
	}
	plugin, err := Plugin.Factory().Build(node)
	if err != nil {
		t.Fatal(err)
	}
	return plugin
}

func dispatch(app *absdk.App, userID, groupID, text string) error {
	return app.Dispatch(context.Background(), &absdk.Event{
		Protocol: "test",
		SelfID:   "bot",
		Type:     "message",
		UserID:   userID,
		GroupID:  groupID,
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
