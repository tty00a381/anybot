package sdk

import (
	"context"
	"reflect"
	"testing"

	"github.com/tty00a381/anybot/core"
)

const testProtocol Protocol = "test"

func TestRuntimeRulesSupportDialogueRouting(t *testing.T) {
	app := NewApp()
	var topic string
	app.OnMessage(All(Private(), NotFromSelf(), RegexRule(`^问\s+(?P<topic>.+)$`))).
		Handle(func(c *EventContext) error {
			topic = c.VarString("topic")
			return nil
		})

	if err := app.Dispatch(context.Background(), &core.Event{
		Protocol: testProtocol,
		SelfID:   "bot",
		UserID:   "bot",
		Type:     "message",
		Text:     "问 天气",
	}); err != nil {
		t.Fatal(err)
	}
	if topic != "" {
		t.Fatalf("self message should be ignored, got %q", topic)
	}

	if err := app.Dispatch(context.Background(), &core.Event{
		Protocol: testProtocol,
		SelfID:   "bot",
		UserID:   "user",
		GroupID:  "group",
		Type:     "message",
		Text:     "问 天气",
	}); err != nil {
		t.Fatal(err)
	}
	if topic != "" {
		t.Fatalf("group message should be ignored, got %q", topic)
	}

	if err := app.Dispatch(context.Background(), &core.Event{
		Protocol: testProtocol,
		SelfID:   "bot",
		UserID:   "user",
		Type:     "message",
		Text:     "问 天气",
	}); err != nil {
		t.Fatal(err)
	}
	if topic != "天气" {
		t.Fatalf("topic = %q", topic)
	}
}

func TestRuntimeExposesCustomRuleBuildingBlocks(t *testing.T) {
	app := NewApp()
	var called bool
	configuredRule := RuleFunc(func(_ context.Context, c *EventContext) (Match, bool) {
		if c.UserID() != "42" {
			return Match{}, false
		}
		return Match{Reason: "configured", Score: 10}.WithVar("persona", "admin"), true
	})
	app.OnMessage(configuredRule).Handle(func(c *EventContext) error {
		called = c.VarString("persona") == "admin"
		return ErrStop
	})
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", UserID: "42", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("custom sdk rule did not run")
	}
}

func TestAllowedGroupsSupportsConfigSlices(t *testing.T) {
	app := NewApp()
	var hits []string
	app.OnMessage(AllowedGroups("100", " 200 ", "100", "")).Handle(func(c *EventContext) error {
		hits = append(hits, c.GroupID())
		return nil
	})
	for _, group := range []string{"100", "200", "300", ""} {
		if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "u", GroupID: group, Text: "hi"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) != 3 || hits[0] != "100" || hits[1] != "200" || hits[2] != "" {
		t.Fatalf("hits = %#v", hits)
	}

	app = NewApp()
	hits = nil
	app.OnMessage(AllowedGroups()).Handle(func(c *EventContext) error {
		hits = append(hits, c.GroupID())
		return nil
	})
	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "u", GroupID: "300", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0] != "300" {
		t.Fatalf("empty allow list should pass, hits = %#v", hits)
	}
}

func TestRequireAdminFallsBackToHostSuperUsers(t *testing.T) {
	app := NewApp(WithSuperUsers("root"))
	var hits int
	var denied error
	app.OnError(func(_ *EventContext, err error) {
		denied = err
	})
	app.Command("admin").Use(RequireAdmin()).Handle(func(*EventContext) error {
		hits++
		return nil
	})
	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "root", Text: "/admin"}); err != nil {
		t.Fatal(err)
	}
	if hits != 1 || denied != nil {
		t.Fatalf("hits=%d denied=%v", hits, denied)
	}
	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "guest", Text: "/admin"}); err != nil {
		t.Fatal(err)
	}
	if hits != 1 || denied == nil {
		t.Fatalf("plugin should deny guest: hits=%d denied=%v", hits, denied)
	}
}

func TestRequireAdminUsesConfiguredAdmins(t *testing.T) {
	app := NewApp(WithSuperUsers("root"))
	var hits int
	var denied error
	app.OnError(func(_ *EventContext, err error) {
		denied = err
	})
	app.Command("admin").Use(RequireAdmin("plugin-admin")).Handle(func(*EventContext) error {
		hits++
		return nil
	})
	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "root", Text: "/admin"}); err != nil {
		t.Fatal(err)
	}
	if hits != 0 || denied == nil {
		t.Fatalf("host superuser should not bypass explicit plugin admins: hits=%d denied=%v", hits, denied)
	}
	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "plugin-admin", Text: "/admin"}); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d", hits)
	}
}

func TestPermissionRulesSupportGroupRolesAndSuperUser(t *testing.T) {
	app := NewApp(WithSuperUsers("root"))
	var hits []string
	app.Command("group").Use(RequirePermission("group.manager")).Handle(func(c *EventContext) error {
		hits = append(hits, "group:"+c.GroupRole())
		return nil
	})
	app.Command("root").Use(RequirePermission("superuser")).Handle(func(c *EventContext) error {
		hits = append(hits, "root:"+c.UserID())
		return nil
	})

	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "member", GroupID: "100", GroupRole: "member", Text: "/group"}); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("member should not pass group.manager: %#v", hits)
	}
	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "manager", GroupID: "100", GroupRole: "manager", Text: "/group"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "owner", GroupID: "100", GroupRole: "owner", Text: "/group"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Dispatch(context.Background(), &Event{Type: "message", UserID: "root", Text: "/root"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"group:manager", "group:owner", "root:root"}
	if !reflect.DeepEqual(hits, want) {
		t.Fatalf("hits = %#v want %#v", hits, want)
	}
}
