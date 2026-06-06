package sdk

import (
	"context"
	"testing"

	"github.com/tty00a381/anybot/core"
)

func TestRuntimeRulesSupportDialogueRouting(t *testing.T) {
	app := core.New()
	var topic string
	app.OnMessage(All(Private(), NotFromSelf(), RegexRule(`^问\s+(?P<topic>.+)$`))).
		Handle(func(c *EventContext) error {
			topic = c.String("topic")
			return nil
		})

	if err := app.Dispatch(context.Background(), &core.Event{
		Protocol: core.ProtocolOneBot11,
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
		Protocol: core.ProtocolOneBot11,
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
		Protocol: core.ProtocolOneBot11,
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
	app := core.New()
	var called bool
	configuredRule := RuleFunc(func(_ context.Context, c *EventContext) (Match, bool) {
		if c.UserID() != "42" {
			return Match{}, false
		}
		return Match{Reason: "configured", Score: 10}.WithVar("persona", "admin"), true
	})
	app.OnMessage(configuredRule).Handle(func(c *EventContext) error {
		called = c.String("persona") == "admin"
		return ErrStop
	})
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", UserID: "42", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("custom sdk rule did not run")
	}
}
