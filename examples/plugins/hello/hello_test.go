package hello

import (
	"testing"

	"github.com/tty00a381/anybot/sdk/testkit"
)

func TestPluginRepliesToCommand(t *testing.T) {
	app := testkit.NewApp()
	if err := app.InstallDefault(Plugin); err != nil {
		t.Fatal(err)
	}
	if err := app.DispatchText("/hello"); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "你好，我是 AnyBot 插件。" {
		t.Fatalf("reply = %q", got)
	}
}
