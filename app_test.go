package anybot

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/tty00a381/anybot/message"
)

func TestRouterPriorityMiddlewareAndStop(t *testing.T) {
	app := New()
	var calls []string
	app.Use(func(next Handler) Handler {
		return func(c *Context) error {
			calls = append(calls, "middleware")
			return next(c)
		}
	})
	app.OnMessage(Any()).Handle(func(c *Context) error {
		calls = append(calls, "low")
		return nil
	})
	app.OnMessage(Any()).Priority(10).Handle(func(c *Context) error {
		calls = append(calls, "high")
		c.Stop()
		return nil
	})

	err := app.dispatch(context.Background(), &Event{Type: "message", Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"middleware", "high"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRoutePriorityInvalidatesSnapshot(t *testing.T) {
	app := New()
	var calls []string
	app.OnMessage(Any()).Handle(func(c *Context) error {
		calls = append(calls, "first")
		return nil
	})
	second := app.OnMessage(Any()).Handle(func(c *Context) error {
		calls = append(calls, "second")
		return nil
	})

	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "hello"})
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("calls=%#v", calls)
	}
	calls = calls[:0]

	second.Priority(10)
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "hello"})
	if !reflect.DeepEqual(calls, []string{"second", "first"}) {
		t.Fatalf("calls=%#v", calls)
	}
}

func TestDispatchPublicEntry(t *testing.T) {
	app := New()
	var got string
	app.Command("ping").Handle(func(c *Context) error {
		got = c.Command()
		return nil
	})
	if err := app.Dispatch(context.Background(), &Event{Type: "message", Text: "/ping"}); err != nil {
		t.Fatal(err)
	}
	if got != "ping" {
		t.Fatalf("got = %q", got)
	}
}

func TestCommandRuleVars(t *testing.T) {
	app := New()
	var command, args string
	var argv []string
	app.Command("echo").Handle(func(c *Context) error {
		command = c.Command()
		args = c.Args()
		argv = c.Argv()
		return nil
	})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "/echo hello world"})
	if command != "echo" || args != "hello world" || !reflect.DeepEqual(argv, []string{"hello", "world"}) {
		t.Fatalf("command=%q args=%q argv=%#v", command, args, argv)
	}
}

func TestCommandRuleQuotedArgv(t *testing.T) {
	app := New()
	var argv []string
	app.Command("echo").Handle(func(c *Context) error {
		argv = c.Argv()
		return nil
	})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "/echo \"hello world\" 'qq bot' a\\ b \"\""})
	want := []string{"hello world", "qq bot", "a b", ""}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv=%#v want=%#v", argv, want)
	}
}

func TestToMeAndMentionedRules(t *testing.T) {
	app := New()
	var calls []string
	app.OnMessage(ToMe()).Handle(func(c *Context) error {
		calls = append(calls, "to_me")
		return nil
	})
	app.OnMessage(Mentioned("7")).Handle(func(c *Context) error {
		calls = append(calls, "mentioned")
		return nil
	})
	event := &Event{
		Protocol: ProtocolOneBot11,
		SelfID:   "7",
		Type:     "message",
		UserID:   "42",
		GroupID:  "100",
		Message:  message.New(message.At("7"), message.Text(" ping")),
	}
	_ = app.dispatch(context.Background(), event)
	want := []string{"to_me", "mentioned"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%#v want=%#v", calls, want)
	}
}

func TestFromSelfRules(t *testing.T) {
	app := New()
	var calls []string
	app.OnMessage(FromSelf()).Handle(func(c *Context) error {
		calls = append(calls, "self:"+c.SelfID())
		return nil
	})
	app.OnMessage(NotFromSelf()).Handle(func(c *Context) error {
		calls = append(calls, "other:"+c.UserID())
		return nil
	})

	_ = app.dispatch(context.Background(), &Event{
		Protocol: ProtocolOneBot11,
		SelfID:   "7",
		Type:     "message",
		UserID:   "7",
		Text:     "self",
	})
	_ = app.dispatch(context.Background(), &Event{
		Protocol: ProtocolOneBot11,
		SelfID:   "7",
		Type:     "message",
		UserID:   "42",
		Text:     "other",
	})
	want := []string{"self:7", "other:42"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%#v want=%#v", calls, want)
	}
}

func TestRuleCombinators(t *testing.T) {
	app := New()
	var calls []string
	var rest string
	app.OnMessage(All(Contains("hello"), Prefix("hello"))).Handle(func(c *Context) error {
		calls = append(calls, "all")
		rest = c.String("rest")
		return nil
	})
	app.OnMessage(AnyOf(Contains("missing"), Contains("world"))).Handle(func(c *Context) error {
		calls = append(calls, "any_of")
		return nil
	})
	app.OnMessage(Not(Contains("blocked"))).Handle(func(c *Context) error {
		calls = append(calls, "not")
		return nil
	})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "hello world"})
	want := []string{"all", "any_of", "not"}
	if !reflect.DeepEqual(calls, want) || rest != "world" {
		t.Fatalf("calls=%#v rest=%q", calls, rest)
	}
}

func TestRouteDefaultNames(t *testing.T) {
	app := New()
	var names []string
	app.Command("echo").Handle(func(c *Context) error {
		names = append(names, c.Route().name)
		return nil
	})
	app.Regex(`^hi$`).Name("custom-regex").Handle(func(c *Context) error {
		names = append(names, c.Route().name)
		return nil
	})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "/echo"})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "hi"})
	want := []string{"command:echo", "custom-regex"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("names=%#v want=%#v", names, want)
	}
}

func TestDefaultRegexRoute(t *testing.T) {
	ResetDefault()
	defer ResetDefault()

	var matched bool
	Regex(`^hi$`).Handle(func(c *Context) error {
		matched = true
		return nil
	})
	_ = Default().dispatch(context.Background(), &Event{Type: "message", Text: "hi"})
	if !matched {
		t.Fatal("default regex route did not match")
	}
}

func TestPassAllowsLaterRoutes(t *testing.T) {
	app := New()
	var calls []string
	app.OnMessage(Any()).Priority(2).Handle(func(c *Context) error {
		calls = append(calls, "first")
		return c.Pass()
	})
	app.OnMessage(Any()).Priority(1).Handle(func(c *Context) error {
		calls = append(calls, "second")
		return nil
	})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "hello"})
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestLifecycleHooksOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter := &fakeBlockingAdapter{}
	app := New(WithAdapter(adapter), WithWorkers(0))
	var calls []string
	app.OnStart(func(context.Context) error {
		calls = append(calls, "start")
		return nil
	})
	app.OnReady(func(context.Context) error {
		calls = append(calls, "ready")
		cancel()
		return nil
	})
	app.OnShutdown(func(context.Context) error {
		calls = append(calls, "shutdown-1")
		return nil
	})
	app.OnShutdown(func(context.Context) error {
		calls = append(calls, "shutdown-2")
		return nil
	})
	err := app.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	want := []string{"start", "ready", "shutdown-2", "shutdown-1"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRecoverReportsPanic(t *testing.T) {
	var got error
	app := New(WithErrorHandler(func(_ *Context, err error) {
		got = err
	}))
	app.Use(Recover())
	app.OnMessage(Any()).Handle(func(*Context) error {
		panic("boom")
	})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "hello"})
	var panicErr *PanicError
	if !errors.As(got, &panicErr) {
		t.Fatalf("got = %#v", got)
	}
}

func TestOnErrorHandlersOrder(t *testing.T) {
	app := New()
	var calls []string
	app.OnError(func(c *Context, err error) {
		calls = append(calls, "first:"+c.RouteName()+":"+err.Error())
	})
	app.OnError(func(c *Context, err error) {
		calls = append(calls, "second:"+c.RouteName()+":"+err.Error())
	})
	app.Command("boom").Handle(func(*Context) error {
		return errors.New("bad")
	})

	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "/boom"})
	want := []string{"first:command:boom:bad", "second:command:boom:bad"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%#v want=%#v", calls, want)
	}
}

func TestMatchVarsDoNotLeakBetweenRoutes(t *testing.T) {
	app := New()
	var args string
	app.Command("echo").Priority(2).Handle(func(c *Context) error {
		return c.Pass()
	})
	app.OnMessage(Any()).Priority(1).Handle(func(c *Context) error {
		args = c.Args()
		return nil
	})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "/echo secret"})
	if args != "" {
		t.Fatalf("args leaked: %q", args)
	}
}

func TestBuiltInMiddlewares(t *testing.T) {
	app := New()
	var hits int
	app.Command("admin").Use(OnlyGroup(), SuperUser("42"), RateLimit(1, time.Minute)).Handle(func(c *Context) error {
		hits++
		return nil
	})
	event := &Event{Type: "message", Text: "/admin", UserID: "42", GroupID: "100"}
	_ = app.dispatch(context.Background(), event)
	_ = app.dispatch(context.Background(), event)
	if hits != 1 {
		t.Fatalf("hits = %d", hits)
	}
}

func TestRateLimitByCustomKey(t *testing.T) {
	app := New()
	var hits int
	app.Command("ping").Use(RateLimitBy(1, time.Minute, func(c *Context) string {
		return c.UserSession().Key()
	})).Handle(func(c *Context) error {
		hits++
		return nil
	})
	_ = app.dispatch(context.Background(), &Event{Protocol: ProtocolOneBot11, Type: "message", Text: "/ping", UserID: "1", GroupID: "100"})
	_ = app.dispatch(context.Background(), &Event{Protocol: ProtocolOneBot11, Type: "message", Text: "/ping", UserID: "1", GroupID: "200"})
	_ = app.dispatch(context.Background(), &Event{Protocol: ProtocolOneBot11, Type: "message", Text: "/ping", UserID: "2", GroupID: "100"})
	if hits != 2 {
		t.Fatalf("hits = %d", hits)
	}
}

func TestSerialByConversation(t *testing.T) {
	app := New(WithSerialByConversation())
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	app.OnMessage(Any()).Handle(func(c *Context) error {
		entered <- struct{}{}
		<-release
		return nil
	})
	event := &Event{Protocol: ProtocolOneBot11, Type: "message", UserID: "42", GroupID: "100"}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = app.dispatch(context.Background(), event)
	}()
	<-entered
	go func() {
		defer wg.Done()
		_ = app.dispatch(context.Background(), event)
	}()
	select {
	case <-entered:
		t.Fatal("second event entered before first event finished")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	wg.Wait()
}

func TestReplyUsesActionClient(t *testing.T) {
	client := &fakeClient{}
	app := New(WithAdapter(fakeAdapter{client: client}))
	c := newContext(context.Background(), app, &Event{
		Protocol: ProtocolOneBot11,
		Type:     "message",
		UserID:   "42",
		Text:     "ping",
		Data:     "raw",
	})
	receipt, err := c.ReplyText("pong")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ID != "99" {
		t.Fatalf("receipt id = %q", receipt.ID)
	}
	if client.target.UserID != "42" || client.sent.Text() != "pong" {
		t.Fatalf("target=%#v sent=%#v", client.target, client.sent)
	}
	if c.ConversationID() != "onebot11:private:42" || c.RawEvent() != "raw" {
		t.Fatalf("conversation=%q raw=%#v", c.ConversationID(), c.RawEvent())
	}
}

type fakeAdapter struct {
	client *fakeClient
}

func (fakeAdapter) Protocol() Protocol { return ProtocolOneBot11 }
func (fakeAdapter) Start(context.Context, EmitFunc) error {
	return errors.New("not used")
}
func (a fakeAdapter) Client() ActionClient { return a.client }

type fakeClient struct {
	target ReplyTarget
	sent   message.Chain
}

func (*fakeClient) Call(context.Context, string, any, any) error { return nil }
func (*fakeClient) CallRaw(context.Context, string, any) (*ActionResponse, error) {
	return &ActionResponse{}, nil
}
func (c *fakeClient) Send(_ context.Context, target ReplyTarget, chain message.Chain) (MessageReceipt, error) {
	c.target = target
	c.sent = chain
	return MessageReceipt{ID: "99"}, nil
}

type fakeBlockingAdapter struct {
}

func (*fakeBlockingAdapter) Protocol() Protocol { return ProtocolOneBot11 }
func (a *fakeBlockingAdapter) Start(ctx context.Context, _ EmitFunc) error {
	<-ctx.Done()
	return ctx.Err()
}
func (*fakeBlockingAdapter) Client() ActionClient { return nil }
