package dialogue

import (
	"testing"

	"github.com/tty00a381/anybot/sdk/testkit"
)

func TestDialogueSavesProfile(t *testing.T) {
	app := testkit.NewApp()
	if err := app.InstallDefault(Plugin); err != nil {
		t.Fatal(err)
	}

	dispatch(t, app, "/whoami")
	if got := app.LastReplyText(); got != "我还不认识你。先发送 /profile 建档。" {
		t.Fatalf("empty profile reply = %q", got)
	}
	dispatch(t, app, "/profile")
	if got := app.LastReplyText(); got != "你想让我怎么称呼你？" {
		t.Fatalf("name prompt = %q", got)
	}
	dispatch(t, app, "小安")
	if got := app.LastReplyText(); got != "你常在哪个城市出没？" {
		t.Fatalf("city prompt = %q", got)
	}
	dispatch(t, app, "杭州")
	if got := app.LastReplyText(); got != "已记住：小安，杭州。" {
		t.Fatalf("saved reply = %q", got)
	}
	dispatch(t, app, "/whoami")
	if got := app.LastReplyText(); got != "你是 小安，常在 杭州。" {
		t.Fatalf("summary reply = %q", got)
	}
}

func dispatch(t *testing.T, app *testkit.App, text string) {
	t.Helper()
	if err := app.DispatchText(text); err != nil {
		t.Fatal(err)
	}
}
