package testkit

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tty00a381/anybot/sdk"
	"github.com/tty00a381/anybot/sdk/message"
)

func TestAppDispatchRecordsReplies(t *testing.T) {
	plugin := sdk.Define(
		sdk.Manifest{Name: "hello"},
		struct{}{},
		func(ctx *sdk.Context, _ struct{}) error {
			ctx.Command("hello").Handle(func(c *sdk.EventContext) error {
				_, err := c.ReplyText(c.UserID() + ":" + c.GroupID())
				return err
			})
			return nil
		},
	)
	app := NewApp()
	if err := app.InstallDefault(plugin); err != nil {
		t.Fatal(err)
	}
	if err := app.DispatchText("/hello", FromUser("alice"), InGroup("100"), WithSelfID("bot")); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "alice:100" {
		t.Fatalf("reply = %q", got)
	}
	sent := app.Client().Sent()
	if len(sent) != 1 || sent[0].Chain.Text() != "alice:100" {
		t.Fatalf("sent = %#v", sent)
	}
	sent[0].Chain = message.New(message.Text("mutated"))
	if got := app.LastReplyText(); got != "alice:100" {
		t.Fatalf("client state was mutated through Sent copy: %q", got)
	}
}

func TestDispatchTextSupportsMentionRules(t *testing.T) {
	plugin := sdk.Define(
		sdk.Manifest{Name: "mention"},
		struct{}{},
		func(ctx *sdk.Context, _ struct{}) error {
			ctx.OnMessage(sdk.ToMe()).Handle(func(c *sdk.EventContext) error {
				_, err := c.ReplyText("mentioned")
				return err
			})
			return nil
		},
	)
	app := NewApp()
	if err := app.InstallDefault(plugin); err != nil {
		t.Fatal(err)
	}
	if err := app.DispatchText("hi", InGroup("100"), WithSelfID("bot")); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "" {
		t.Fatalf("unmentioned group message should not reply, got %q", got)
	}
	if err := app.DispatchText("hi", InGroup("100"), WithSelfID("bot"), MentionSelf()); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "mentioned" {
		t.Fatalf("reply = %q", got)
	}
}

func TestWithMessageClonesChain(t *testing.T) {
	original := message.New(message.Text("stable"))
	event := Message("", WithMessage(original))
	original[0].Data["text"] = "mutated"
	if got := event.Message.Text(); got != "stable" {
		t.Fatalf("event message should be cloned, got %q", got)
	}
}

func TestAppProvidesHostCapabilities(t *testing.T) {
	store := &configStore{}
	root := t.TempDir()
	pluginID := "plg_bbbbbbbbbbbbbbbbbbbbbbbbbb"
	plugin := sdk.Define(
		sdk.Manifest{Name: "hosted"},
		struct{}{},
		func(ctx *sdk.Context, _ struct{}) error {
			dir, err := ctx.DataDir()
			if err != nil {
				return err
			}
			if dir != filepath.Join(root, "plugins", pluginID) {
				t.Fatalf("data dir = %q", dir)
			}
			return ctx.Config().Set(context.Background(), "command", "hosted")
		},
	)
	app := NewApp(WithPluginID(pluginID), WithDataDir(root), WithConfigStore(store))
	if err := app.InstallDefault(plugin); err != nil {
		t.Fatal(err)
	}
	if store.plugin != pluginID || !reflect.DeepEqual(store.assignments, []sdk.ConfigAssignment{
		{Path: []string{"command"}, Value: "hosted"},
	}) {
		t.Fatalf("plugin=%q assignments=%#v", store.plugin, store.assignments)
	}
}

func TestDispatchRejectsNilApp(t *testing.T) {
	var app *App
	if err := app.DispatchText("/hello"); err != ErrAppUnavailable {
		t.Fatalf("err = %v", err)
	}
}

type configStore struct {
	plugin      string
	assignments []sdk.ConfigAssignment
	resets      [][]string
}

func (s *configStore) SetPluginConfig(_ context.Context, plugin string, assignments []sdk.ConfigAssignment) error {
	s.plugin = plugin
	s.assignments = append([]sdk.ConfigAssignment(nil), assignments...)
	return nil
}

func (s *configStore) ResetPluginConfig(_ context.Context, plugin string, paths [][]string) error {
	s.plugin = plugin
	s.resets = append([][]string(nil), paths...)
	return nil
}
