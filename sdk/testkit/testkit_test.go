package testkit

import (
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

func TestDispatchRejectsNilApp(t *testing.T) {
	var app *App
	if err := app.DispatchText("/hello"); err != ErrAppUnavailable {
		t.Fatalf("err = %v", err)
	}
}
