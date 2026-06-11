package groupmemo

import (
	"testing"

	"github.com/tty00a381/anybot/sdk/testkit"
)

func TestGroupMemoStoresPerGroup(t *testing.T) {
	app := testkit.NewApp(testkit.WithSuperUsers("root"))
	if err := app.InstallDefault(Plugin); err != nil {
		t.Fatal(err)
	}

	if err := app.DispatchText("/remember 晚上八点开黑", testkit.FromUser("root"), testkit.InGroup("100")); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "群便签已更新。" {
		t.Fatalf("set reply = %q", got)
	}
	if err := app.DispatchText("/memo", testkit.FromUser("alice"), testkit.InGroup("100")); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "晚上八点开黑" {
		t.Fatalf("memo reply = %q", got)
	}
	if err := app.DispatchText("/memo", testkit.FromUser("alice"), testkit.InGroup("200")); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "这个群还没有便签。" {
		t.Fatalf("other group reply = %q", got)
	}
}

func TestGroupMemoAllowedGroups(t *testing.T) {
	app := testkit.NewApp(testkit.WithSuperUsers("root"))
	if err := testkit.InstallConfig(app, Plugin, Config{
		ViewCommand:   "memo",
		SetCommand:    "remember",
		AllowedGroups: []string{"100"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.DispatchText("/remember 不应该写入", testkit.FromUser("root"), testkit.InGroup("200")); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "" {
		t.Fatalf("disallowed group should not reply, got %q", got)
	}
	if err := app.DispatchText("/remember 可以写入", testkit.FromUser("root"), testkit.InGroup("100")); err != nil {
		t.Fatal(err)
	}
	if got := app.LastReplyText(); got != "群便签已更新。" {
		t.Fatalf("allowed group reply = %q", got)
	}
}
