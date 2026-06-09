package sdk

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tty00a381/anybot/core"
	coremsg "github.com/tty00a381/anybot/core/message"
)

func TestDialogueCapturesAndCompletesConversation(t *testing.T) {
	client := &dialogueTestClient{}
	app := NewApp(WithAdapter(dialogueTestAdapter{client: client}))
	ctx := NewContext(app, Manifest{Name: "profile"}, WithPluginID(sdkTestProfileID))
	dialogue := ctx.Dialogue("signup")
	dialogue.Step("ask-name", func(turn *DialogueTurn) error {
		name := turn.Text()
		if name == "" {
			_, err := turn.ReplyText("名字不能为空")
			return err
		}
		return turn.EndText("你好，" + name)
	})

	ctx.Command("signup").Handle(func(c *EventContext) error {
		return dialogue.BeginText(c, "ask-name", map[string]string{"source": "command"}, "你叫什么名字？")
	})

	var fallbackCalled bool
	ctx.OnMessage(Any()).Handle(func(*EventContext) error {
		fallbackCalled = true
		return nil
	})

	dispatch(t, app, "user", "/signup")
	if client.lastText() != "你叫什么名字？" {
		t.Fatalf("prompt = %q", client.lastText())
	}
	if fallbackCalled {
		t.Fatal("fallback should not run for command handled route")
	}

	dispatch(t, app, "user", "小安")
	if client.lastText() != "你好，小安" {
		t.Fatalf("completion reply = %q", client.lastText())
	}
	if fallbackCalled {
		t.Fatal("fallback should be stopped while dialogue is active")
	}

	dispatch(t, app, "user", "闲聊")
	if !fallbackCalled {
		t.Fatal("fallback should run after dialogue state is cleared")
	}
}

func TestDialoguePassesWhenNoConversationIsActive(t *testing.T) {
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "dialogue"}, WithPluginID(sdkTestDialogueID))
	ctx.Dialogue("flow").
		Step("next", func(*DialogueTurn) error {
			t.Fatal("inactive dialogue should not call step handler")
			return nil
		})
	var called bool
	ctx.OnMessage(Any()).Handle(func(*EventContext) error {
		called = true
		return nil
	})

	dispatch(t, app, "user", "hello")
	if !called {
		t.Fatal("normal message route did not run")
	}
}

func TestDialogueKeepsPluginStateIsolated(t *testing.T) {
	app := NewApp()
	first := NewContext(app, Manifest{Name: "first"}, WithPluginID(sdkTestFirstID))
	second := NewContext(app, Manifest{Name: "second"}, WithPluginID(sdkTestSecondID))
	firstDialogue := first.Dialogue("flow")
	secondDialogue := second.Dialogue("flow")
	firstDialogue.Step("step", func(turn *DialogueTurn) error {
		return turn.End()
	})
	secondDialogue.Step("step", func(turn *DialogueTurn) error {
		t.Fatal("second plugin dialogue should not handle first plugin state")
		return nil
	})

	ctx := NewTestContext(app, &core.Event{Protocol: testProtocol, Type: "message", UserID: "42"})
	if err := firstDialogue.Begin(ctx, "step", map[string]string{"plugin": "first"}); err != nil {
		t.Fatal(err)
	}
	if snapshot, ok, err := firstDialogue.Active(ctx); err != nil || !ok || snapshot.Step != "step" {
		t.Fatalf("first active state: ok=%v err=%v snapshot=%#v", ok, err, snapshot)
	}
	if _, ok, err := secondDialogue.Active(ctx); err != nil || ok {
		t.Fatalf("second should not see first state: ok=%v err=%v", ok, err)
	}
}

func TestGroupDialogueRequiresGroupContext(t *testing.T) {
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "dialogue"}, WithPluginID(sdkTestDialogueID))
	dialogue := ctx.Dialogue("group", DialogueWithScope(DialogueScopeGroup)).
		Step("next", func(*DialogueTurn) error {
			t.Fatal("private event should not enter group dialogue")
			return nil
		})
	private := NewTestContext(app, &core.Event{Protocol: testProtocol, Type: "message", UserID: "42"})
	if err := dialogue.Begin(private, "next", nil); !errors.Is(err, ErrGroupContextUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestGroupDialogueIgnoresPrivateMessagesWhenInactive(t *testing.T) {
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "dialogue"}, WithPluginID(sdkTestDialogueID))
	ctx.Dialogue("group", DialogueWithScope(DialogueScopeGroup)).
		Step("next", func(*DialogueTurn) error {
			t.Fatal("private event should not enter group dialogue")
			return nil
		})
	var called bool
	ctx.OnMessage(Any()).Handle(func(*EventContext) error {
		called = true
		return nil
	})
	dispatch(t, app, "user", "hello")
	if !called {
		t.Fatal("private message should continue past inactive group dialogue")
	}
}

func TestDialogueTurnCanCarryStateAcrossSteps(t *testing.T) {
	client := &dialogueTestClient{}
	app := NewApp(WithAdapter(dialogueTestAdapter{client: client}))
	ctx := NewContext(app, Manifest{Name: "survey"}, WithPluginID(sdkTestSurveyID))
	dialogue := ctx.Dialogue("survey", DialogueWithTTL(time.Hour))
	type answers struct {
		Name string `json:"name"`
	}
	dialogue.
		Step("name", func(turn *DialogueTurn) error {
			return turn.NextText("city", answers{Name: turn.Text()}, "你在哪个城市？")
		}).
		Step("city", func(turn *DialogueTurn) error {
			var state answers
			if err := turn.Load(&state); err != nil {
				return err
			}
			return turn.EndText(state.Name + " 在 " + turn.Text())
		})

	ctx.Command("survey").Handle(func(c *EventContext) error {
		return dialogue.BeginText(c, "name", nil, "你叫什么？")
	})

	dispatch(t, app, "user", "/survey")
	dispatch(t, app, "user", "小安")
	if client.lastText() != "你在哪个城市？" {
		t.Fatalf("second prompt = %q", client.lastText())
	}
	dispatch(t, app, "user", "上海")
	if client.lastText() != "小安 在 上海" {
		t.Fatalf("summary = %q", client.lastText())
	}
}

func TestDialogueRejectsUnknownSteps(t *testing.T) {
	app := NewApp()
	var handledErr error
	app.OnError(func(_ *EventContext, err error) {
		handledErr = err
	})
	ctx := NewContext(app, Manifest{Name: "dialogue"}, WithPluginID(sdkTestDialogueID))
	dialogue := ctx.Dialogue("flow")
	var fallbackCalled bool
	ctx.OnMessage(Any()).Handle(func(*EventContext) error {
		fallbackCalled = true
		return nil
	})
	event := NewTestContext(app, &core.Event{Type: "message", UserID: "42"})
	if err := dialogue.Begin(event, "missing", nil); err == nil {
		t.Fatal("Begin should reject unregistered steps")
	}
	dialogue.Step("first", func(turn *DialogueTurn) error {
		return turn.Next("missing", nil)
	})
	if err := dialogue.Begin(event, "first", nil); err != nil {
		t.Fatal(err)
	}
	if err := app.Dispatch(context.Background(), event.Event()); err != nil {
		t.Fatal(err)
	}
	if handledErr == nil || !strings.Contains(handledErr.Error(), `step "missing" is not registered`) {
		t.Fatalf("handledErr = %v", handledErr)
	}
	if fallbackCalled {
		t.Fatal("dialogue errors should not fall through to normal message routes")
	}
}

func dispatch(t *testing.T, app *App, userID, text string) {
	t.Helper()
	if err := app.Dispatch(context.Background(), &core.Event{
		Protocol: testProtocol,
		SelfID:   "bot",
		Type:     "message",
		UserID:   userID,
		Text:     text,
	}); err != nil {
		t.Fatal(err)
	}
}

type dialogueTestAdapter struct {
	client ActionClient
}

func (a dialogueTestAdapter) Protocol() Protocol                         { return testProtocol }
func (a dialogueTestAdapter) Start(context.Context, core.EmitFunc) error { return nil }
func (a dialogueTestAdapter) Client() ActionClient                       { return a.client }

type dialogueTestClient struct {
	sent []coremsg.Chain
}

func (c *dialogueTestClient) Send(_ context.Context, _ ReplyTarget, chain coremsg.Chain) (MessageReceipt, error) {
	c.sent = append(c.sent, chain.Clone())
	return MessageReceipt{ID: "sent"}, nil
}

func (c *dialogueTestClient) lastText() string {
	if len(c.sent) == 0 {
		return ""
	}
	return c.sent[len(c.sent)-1].Text()
}
