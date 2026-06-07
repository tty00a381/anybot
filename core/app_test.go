package core

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tty00a381/anybot/core/message"
)

const testProtocol Protocol = "test"

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

func TestGroupMiddlewareAppliesToExistingRoutes(t *testing.T) {
	app := New()
	group := app.Group()
	var calls []string
	group.Command("ping").Handle(func(*Context) error {
		calls = append(calls, "handler")
		return nil
	})
	group.Use(func(next Handler) Handler {
		return func(c *Context) error {
			calls = append(calls, "group")
			return next(c)
		}
	})

	if err := app.Dispatch(context.Background(), &Event{Type: "message", Text: "/ping"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"group", "handler"}) {
		t.Fatalf("calls=%#v", calls)
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
		Protocol: testProtocol,
		SelfID:   "7",
		Type:     "message",
		UserID:   "42",
		GroupID:  "100",
		Message:  message.New(message.Raw("at", map[string]any{"id": "7"}), message.Text(" ping")),
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
		Protocol: testProtocol,
		SelfID:   "7",
		Type:     "message",
		UserID:   "7",
		Text:     "self",
	})
	_ = app.dispatch(context.Background(), &Event{
		Protocol: testProtocol,
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
		rest = c.VarString("rest")
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

func TestWaitActionReadyUsesAdapterState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter := &fakeStatefulAdapter{started: make(chan struct{})}
	app := New(WithAdapter(adapter), WithWorkers(0))
	ready := make(chan error, 1)
	app.Go("wait-ready", func(ctx context.Context) error {
		ready <- app.WaitActionReady(ctx)
		return nil
	})
	errc := make(chan error, 1)
	go func() {
		errc <- app.Run(ctx)
	}()
	select {
	case <-adapter.started:
	case <-time.After(time.Second):
		t.Fatal("adapter did not start")
	}
	select {
	case err := <-ready:
		t.Fatalf("ready returned before adapter state: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	adapter.emit(AdapterState{Protocol: testProtocol, Kind: AdapterStateReady, ActionReady: true})
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("wait ready error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait action ready did not return")
	}
	cancel()
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run err = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run did not stop")
	}
}

func TestCriticalTaskStopsRun(t *testing.T) {
	app := New(WithAdapter(&fakeBlockingAdapter{}), WithWorkers(0))
	app.Go("boom", func(context.Context) error {
		return errors.New("boom")
	}, TaskCritical())
	err := app.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "task boom failed") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestObserverIsOutsideRouteSemantics(t *testing.T) {
	app := New(WithObserverBuffer(4))
	observed := make(chan string, 1)
	observerErr := make(chan error, 1)
	var routeErr error
	app.OnError(func(_ *Context, err error) {
		routeErr = err
	})
	app.OnObserverError(func(_ context.Context, _ *Event, err error) {
		observerErr <- err
	})
	app.Observe(MessageEvent()).Name("memory").Handle(func(_ context.Context, event *Event) error {
		observed <- event.Text
		return errors.New("observer failed")
	})
	app.OnMessage(Any()).Handle(func(c *Context) error {
		c.Stop()
		return nil
	})
	if err := app.Dispatch(context.Background(), &Event{Type: "message", Text: "hello"}); err != nil {
		t.Fatal(err)
	}
	select {
	case text := <-observed:
		if text != "hello" {
			t.Fatalf("observed text = %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("observer did not run")
	}
	select {
	case err := <-observerErr:
		if err == nil || err.Error() != "observer failed" {
			t.Fatalf("observer err = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("observer error was not reported")
	}
	if routeErr != nil {
		t.Fatalf("observer error leaked to route handler: %v", routeErr)
	}
}

func TestObserverContextFollowsAppLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter := &fakeEmittingAdapter{started: make(chan struct{})}
	app := New(WithAdapter(adapter), WithWorkers(0), WithObserverBuffer(1))
	values := make(chan any, 1)
	done := make(chan error, 1)
	app.Observe(MessageEvent()).Handle(func(ctx context.Context, _ *Event) error {
		values <- ctx.Value(observerTestContextKey{})
		<-ctx.Done()
		done <- ctx.Err()
		return nil
	})
	errc := make(chan error, 1)
	go func() {
		errc <- app.Run(ctx)
	}()
	select {
	case <-adapter.started:
	case <-time.After(time.Second):
		t.Fatal("adapter did not emit event")
	}
	select {
	case value := <-values:
		if value != "event-value" {
			t.Fatalf("observer context value = %#v", value)
		}
	case <-time.After(time.Second):
		t.Fatal("observer did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("observer context err = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("observer context was not canceled")
	}
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run err = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run did not stop")
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

func TestMatchVarsDoNotPolluteLocalValues(t *testing.T) {
	app := New()
	var commandValue any
	var commandOK bool
	var command string
	app.Command("echo").Handle(func(c *Context) error {
		commandValue, commandOK = c.Get("command")
		command = c.Command()
		c.Set("command", "local")
		if got := c.String("command"); got != "local" {
			t.Fatalf("local command value = %q", got)
		}
		return nil
	})

	if err := app.Dispatch(context.Background(), &Event{Type: "message", Text: "/echo"}); err != nil {
		t.Fatal(err)
	}
	if commandOK || commandValue != nil {
		t.Fatalf("match command polluted locals: ok=%v value=%#v", commandOK, commandValue)
	}
	if command != "echo" {
		t.Fatalf("command = %q", command)
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

func TestRequireSuperUserUsesAppConfig(t *testing.T) {
	app := New(WithSuperUsers("42"))
	var hits int
	var denied error
	app.OnError(func(_ *Context, err error) {
		denied = err
	})
	app.Command("admin").Use(RequireSuperUser()).Handle(func(c *Context) error {
		if !c.IsSuperUser() {
			t.Fatal("context should report superuser")
		}
		hits++
		return nil
	})
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "/admin", UserID: "42"})
	if hits != 1 || denied != nil {
		t.Fatalf("hits=%d denied=%v", hits, denied)
	}
	_ = app.dispatch(context.Background(), &Event{Type: "message", Text: "/admin", UserID: "7"})
	if !errors.Is(denied, ErrUnauthorized) {
		t.Fatalf("denied = %v", denied)
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
	_ = app.dispatch(context.Background(), &Event{Protocol: testProtocol, Type: "message", Text: "/ping", UserID: "1", GroupID: "100"})
	_ = app.dispatch(context.Background(), &Event{Protocol: testProtocol, Type: "message", Text: "/ping", UserID: "1", GroupID: "200"})
	_ = app.dispatch(context.Background(), &Event{Protocol: testProtocol, Type: "message", Text: "/ping", UserID: "2", GroupID: "100"})
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
	event := &Event{Protocol: testProtocol, Type: "message", UserID: "42", GroupID: "100"}
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
		Protocol: testProtocol,
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
	if c.ConversationID() != "test:private:42" || c.RawEvent() != "raw" {
		t.Fatalf("conversation=%q raw=%#v", c.ConversationID(), c.RawEvent())
	}
}

type fakeAdapter struct {
	client *fakeClient
}

func (fakeAdapter) Protocol() Protocol { return testProtocol }
func (fakeAdapter) Start(context.Context, EmitFunc) error {
	return errors.New("not used")
}
func (a fakeAdapter) Client() ActionClient { return a.client }

type fakeClient struct {
	target ReplyTarget
	sent   message.Chain
}

func (c *fakeClient) Send(_ context.Context, target ReplyTarget, chain message.Chain) (MessageReceipt, error) {
	c.target = target
	c.sent = chain
	return MessageReceipt{ID: "99"}, nil
}

type fakeBlockingAdapter struct {
}

func (*fakeBlockingAdapter) Protocol() Protocol { return testProtocol }
func (a *fakeBlockingAdapter) Start(ctx context.Context, _ EmitFunc) error {
	<-ctx.Done()
	return ctx.Err()
}
func (*fakeBlockingAdapter) Client() ActionClient { return nil }

type observerTestContextKey struct{}

type fakeEmittingAdapter struct {
	started chan struct{}
}

func (*fakeEmittingAdapter) Protocol() Protocol { return testProtocol }
func (a *fakeEmittingAdapter) Start(ctx context.Context, emit EmitFunc) error {
	eventCtx := context.WithValue(ctx, observerTestContextKey{}, "event-value")
	_ = emit(eventCtx, &Event{Type: "message", Text: "hello"})
	close(a.started)
	<-ctx.Done()
	return ctx.Err()
}
func (*fakeEmittingAdapter) Client() ActionClient { return nil }

type fakeStatefulAdapter struct {
	sink    AdapterStateSink
	started chan struct{}
}

func (*fakeStatefulAdapter) Protocol() Protocol { return testProtocol }
func (a *fakeStatefulAdapter) Start(ctx context.Context, _ EmitFunc) error {
	close(a.started)
	<-ctx.Done()
	return ctx.Err()
}
func (*fakeStatefulAdapter) Client() ActionClient { return nil }
func (*fakeStatefulAdapter) State() AdapterState {
	return AdapterState{Protocol: testProtocol, Kind: AdapterStateUnknown}
}
func (a *fakeStatefulAdapter) SetStateSink(sink AdapterStateSink) {
	a.sink = sink
}
func (a *fakeStatefulAdapter) emit(state AdapterState) {
	a.sink(context.Background(), state)
}
