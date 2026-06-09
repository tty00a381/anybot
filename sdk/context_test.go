package sdk

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tty00a381/anybot/core"
	coremsg "github.com/tty00a381/anybot/core/message"
)

const (
	sdkTestWeatherID   = "plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"
	sdkTestFirstID     = "plg_bbbbbbbbbbbbbbbbbbbbbbbbbb"
	sdkTestSecondID    = "plg_cccccccccccccccccccccccccc"
	sdkTestRateLimitID = "plg_dddddddddddddddddddddddddd"
	sdkTestProfileID   = "plg_eeeeeeeeeeeeeeeeeeeeeeeeee"
	sdkTestCounterID   = "plg_ffffffffffffffffffffffffff"
	sdkTestAdminID     = "plg_22222222222222222222222222"
	sdkTestDialogueID  = "plg_33333333333333333333333333"
	sdkTestSurveyID    = "plg_44444444444444444444444444"
)

func TestContextLoggerScopesPluginName(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	ctx := NewContext(
		NewApp(WithLogger(logger)),
		Manifest{Name: "weather"},
		WithPluginID(sdkTestWeatherID),
	)
	ctx.Logger().Info("ready")
	if !strings.Contains(out.String(), "plugin_id="+sdkTestWeatherID) || !strings.Contains(out.String(), "plugin_name=weather") {
		t.Fatalf("log output = %q", out.String())
	}
}

func TestContextSendTextUsesActionClient(t *testing.T) {
	client := &sdkTestClient{}
	ctx := NewContext(
		NewApp(WithAdapter(sdkTestAdapter{client: client})),
		Manifest{Name: "dialogue"},
	)
	receipt, err := ctx.SendText(context.Background(), ReplyTarget{Protocol: testProtocol, UserID: "42"}, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ID != "sent" {
		t.Fatalf("receipt = %#v", receipt)
	}
	if client.target.UserID != "42" || client.chain.Text() != "hello" {
		t.Fatalf("target=%#v chain=%#v", client.target, client.chain)
	}
}

func TestContextUseIsPluginScoped(t *testing.T) {
	app := NewApp()
	first := NewContext(app, Manifest{Name: "first"}, WithPluginID(sdkTestFirstID))
	second := NewContext(app, Manifest{Name: "second"}, WithPluginID(sdkTestSecondID))
	var scopedCalls int
	first.Use(func(next Handler) Handler {
		return func(c *EventContext) error {
			scopedCalls++
			return next(c)
		}
	})
	var firstCalled, secondCalled bool
	first.Command("first").Handle(func(*EventContext) error {
		firstCalled = true
		return nil
	})
	second.Command("second").Handle(func(*EventContext) error {
		secondCalled = true
		return nil
	})
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", Text: "/second"}); err != nil {
		t.Fatal(err)
	}
	if !secondCalled || firstCalled || scopedCalls != 0 {
		t.Fatalf("second route leaked plugin middleware: first=%v second=%v scoped=%d", firstCalled, secondCalled, scopedCalls)
	}
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", Text: "/first"}); err != nil {
		t.Fatal(err)
	}
	if !firstCalled || scopedCalls != 1 {
		t.Fatalf("first route did not use plugin middleware: first=%v scoped=%d", firstCalled, scopedCalls)
	}
}

func TestContextUseGlobalRequiresHostGrant(t *testing.T) {
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "ratelimit"}, WithPluginID(sdkTestRateLimitID))
	if err := ctx.UseGlobal(Timeout(time.Second)); !errors.Is(err, ErrGlobalMiddlewareUnavailable) {
		t.Fatalf("err = %v", err)
	}
	ctx = NewContext(app, Manifest{Name: "ratelimit"}, WithEnvironment(Environment{PluginID: sdkTestRateLimitID, AllowGlobalMiddleware: true}))
	if err := ctx.UseGlobal(Timeout(time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestContextRouteNamesArePluginScoped(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	app := NewApp(WithLogger(logger))
	app.Use(Trace(logger))
	ctx := NewContext(app, Manifest{Name: "weather"}, WithPluginID(sdkTestWeatherID))
	ctx.Command("weather").Name("command").Handle(func(*EventContext) error {
		return nil
	})
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", Text: "/weather"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "route="+sdkTestWeatherID+".command") {
		t.Fatalf("route name was not scoped:\n%s", out.String())
	}
}

func TestContextRouteNamesKeepExplicitPluginPrefix(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	app := NewApp(WithLogger(logger))
	app.Use(Trace(logger))
	ctx := NewContext(app, Manifest{Name: "weather"}, WithPluginID(sdkTestWeatherID))
	ctx.Command("weather").Name(sdkTestWeatherID + ".command").Handle(func(*EventContext) error {
		return nil
	})
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", Text: "/weather"}); err != nil {
		t.Fatal(err)
	}
	log := out.String()
	if strings.Contains(log, "route="+sdkTestWeatherID+"."+sdkTestWeatherID+".command") || !strings.Contains(log, "route="+sdkTestWeatherID+".command") {
		t.Fatalf("route name was double-scoped:\n%s", log)
	}
}

func TestContextSessionsArePluginScoped(t *testing.T) {
	app := NewApp()
	first := NewContext(app, Manifest{Name: "first"}, WithPluginID(sdkTestFirstID))
	second := NewContext(app, Manifest{Name: "second"}, WithPluginID(sdkTestSecondID))
	event := NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
	if err := first.UserSession(event).SaveJSON(context.Background(), "profile", map[string]string{"name": "first"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	var profile map[string]string
	if ok, err := second.UserSession(event).LoadJSON(context.Background(), "profile", &profile); err != nil || ok {
		t.Fatalf("second plugin should not see first profile: ok=%v err=%v profile=%#v", ok, err, profile)
	}
	if ok, err := first.UserSession(event).LoadJSON(context.Background(), "profile", &profile); err != nil || !ok || profile["name"] != "first" {
		t.Fatalf("first plugin profile missing: ok=%v err=%v profile=%#v", ok, err, profile)
	}
}

func TestContextUsesPluginIDForStateNamespace(t *testing.T) {
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "天气"}, WithPluginID(sdkTestWeatherID))
	if ctx.PluginID() != sdkTestWeatherID {
		t.Fatalf("plugin id = %q", ctx.PluginID())
	}
	if key := ctx.SessionBy("settings").Key(); key != sdkTestWeatherID+".settings" {
		t.Fatalf("session key = %q", key)
	}
}

func TestInstallDefaultWithIDScopesEmbeddedPluginState(t *testing.T) {
	type config struct {
		Key string
	}
	plugin := Define(Manifest{Name: "memory"}, config{Key: "profile"}, func(ctx *Context, cfg config) error {
		event := NewTestContext(&App{runtime: ctx.app}, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
		return UserState[string](ctx, event, cfg.Key).Save(ctx.PluginID(), time.Hour)
	})
	app := NewApp()
	if err := InstallDefaultWithID(app, sdkTestFirstID, plugin); err != nil {
		t.Fatal(err)
	}
	if err := InstallDefaultWithID(app, sdkTestSecondID, plugin); err != nil {
		t.Fatal(err)
	}
	event := NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
	firstCtx := NewContext(app, Manifest{Name: "first"}, WithPluginID(sdkTestFirstID))
	secondCtx := NewContext(app, Manifest{Name: "second"}, WithPluginID(sdkTestSecondID))
	first, ok, err := UserState[string](firstCtx, event, "profile").Load()
	if err != nil || !ok || first != sdkTestFirstID {
		t.Fatalf("first state=%q ok=%v err=%v", first, ok, err)
	}
	second, ok, err := UserState[string](secondCtx, event, "profile").Load()
	if err != nil || !ok || second != sdkTestSecondID {
		t.Fatalf("second state=%q ok=%v err=%v", second, ok, err)
	}
}

func TestTypedStateUsesEventContext(t *testing.T) {
	type profile struct {
		Name string `json:"name"`
	}
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "profile"}, WithPluginID(sdkTestProfileID))
	event := NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
	state := UserState[profile](ctx, event, "profile")
	if value, ok, err := state.Load(); err != nil || ok || value.Name != "" {
		t.Fatalf("empty load: value=%#v ok=%v err=%v", value, ok, err)
	}
	if err := state.Save(profile{Name: "alice"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	value, ok, err := state.Load()
	if err != nil || !ok || value.Name != "alice" {
		t.Fatalf("saved load: value=%#v ok=%v err=%v", value, ok, err)
	}
	if err := state.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := state.Load(); err != nil || ok {
		t.Fatalf("deleted load: ok=%v err=%v", ok, err)
	}
}

func TestTypedStateUpdateLoadsMutatesAndSaves(t *testing.T) {
	type counter struct {
		Count int `json:"count"`
	}
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "counter"}, WithPluginID(sdkTestCounterID))
	event := NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
	state := UserState[counter](ctx, event, "counter")
	value, err := state.Update(counter{}, time.Hour, func(value *counter) error {
		value.Count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.Count != 1 {
		t.Fatalf("updated value = %#v", value)
	}
	value, ok, err := state.Load()
	if err != nil || !ok || value.Count != 1 {
		t.Fatalf("saved value=%#v ok=%v err=%v", value, ok, err)
	}
	if _, err := state.Update(counter{}, 0, nil); err == nil || !strings.Contains(err.Error(), "update function is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestNamedStateUsesPluginAndScopeNamespace(t *testing.T) {
	type setting struct {
		Value string `json:"value"`
	}
	app := NewApp()
	first := NewContext(app, Manifest{Name: "first"}, WithPluginID(sdkTestFirstID))
	second := NewContext(app, Manifest{Name: "second"}, WithPluginID(sdkTestSecondID))
	event := NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
	if err := NamedState[setting](first, event, "value", "settings").Save(setting{Value: "first"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if value, ok, err := NamedState[setting](second, event, "value", "settings").Load(); err != nil || ok || value.Value != "" {
		t.Fatalf("second plugin should not see first state: value=%#v ok=%v err=%v", value, ok, err)
	}
	if value, ok, err := NamedState[setting](first, event, "value", "settings").Load(); err != nil || !ok || value.Value != "first" {
		t.Fatalf("first state missing: value=%#v ok=%v err=%v", value, ok, err)
	}
}

func TestTypedStateUnavailable(t *testing.T) {
	if _, _, err := UserState[int](nil, nil, "count").Load(); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("err = %v", err)
	}
	ctx := NewContext(nil, Manifest{Name: "counter"}, WithPluginID("plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"))
	if err := UserState[int](ctx, nil, "count").Save(1, 0); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("err = %v", err)
	}
	ctx = NewContext(NewApp(), Manifest{Name: "counter"}, WithPluginID("plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"))
	if err := UserState[int](ctx, nil, "count").Save(1, 0); !errors.Is(err, ErrEventContextUnavailable) {
		t.Fatalf("err = %v", err)
	}
	app := NewApp()
	ctx = NewContext(app, Manifest{Name: "counter"}, WithPluginID("plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"))
	event := NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
	if _, _, err := UserState[int](ctx, event, "").Load(); !errors.Is(err, ErrStateKeyRequired) {
		t.Fatalf("err = %v", err)
	}
	if err := UserState[int](ctx, event, "   ").Save(1, 0); !errors.Is(err, ErrStateKeyRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestGroupStateRequiresGroupContext(t *testing.T) {
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "memo"}, WithPluginID(sdkTestCounterID))
	private := NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
	if err := ctx.GroupSession(private).SaveJSON(private.Context(), "memo", 1, 0); !errors.Is(err, ErrGroupContextUnavailable) {
		t.Fatalf("group session err = %v", err)
	}
	if err := GroupState[int](ctx, private, "memo").Save(1, 0); !errors.Is(err, ErrGroupContextUnavailable) {
		t.Fatalf("err = %v", err)
	}
	group := NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", GroupID: "100", Type: "message"})
	if err := GroupState[int](ctx, group, "memo").Save(1, 0); err != nil {
		t.Fatal(err)
	}
	if value, ok, err := GroupState[int](ctx, group, "memo").Load(); err != nil || !ok || value != 1 {
		t.Fatalf("group value=%d ok=%v err=%v", value, ok, err)
	}
}

func TestContextConfigWritesThroughStore(t *testing.T) {
	store := &sdkConfigStore{}
	app := NewApp()
	ctx := NewContext(app, Manifest{Name: "Minecraft 管理"}, WithPluginID(sdkTestAdminID), WithConfigStore(store))
	if !ctx.Config().Available() {
		t.Fatal("config handle should be available")
	}
	if err := ctx.Config().Set(context.Background(), "bridge.group_to_game", "prefix"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.assignments, []ConfigAssignment{
		{Path: []string{"bridge", "group_to_game"}, Value: "prefix"},
	}) || store.plugin != sdkTestAdminID {
		t.Fatalf("store plugin=%q assignments=%#v", store.plugin, store.assignments)
	}
	if err := ctx.Config().Reset(context.Background(), "bridge.group_to_game"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.resets, [][]string{{"bridge", "group_to_game"}}) {
		t.Fatalf("resets = %#v", store.resets)
	}
	if err := ctx.Config().SetAll(context.Background(), ConfigAssignment{Path: []string{"-bad"}, Value: true}); err == nil {
		t.Fatal("invalid SetAll path should be rejected")
	}
}

func TestContextConfigUnavailableWithoutHostStore(t *testing.T) {
	ctx := NewContext(NewApp(), Manifest{Name: "minecraft"}, WithPluginID(sdkTestAdminID))
	if ctx.Config().Available() {
		t.Fatal("config handle should be unavailable")
	}
	err := ctx.Config().Set(context.Background(), "bridge.group_to_game", "prefix")
	if !errors.Is(err, ErrConfigStoreUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestContextDataDirUsesPluginNamespace(t *testing.T) {
	root := t.TempDir()
	ctx := NewContext(NewApp(), Manifest{Name: "天气"}, WithPluginID("plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"), WithDataDir(root))
	dir, err := ctx.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "plugins", "plg_aaaaaaaaaaaaaaaaaaaaaaaaaa")
	if dir != want {
		t.Fatalf("dir = %q, want %q", dir, want)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
}

func TestContextDataDirUsesPluginID(t *testing.T) {
	root := t.TempDir()
	ctx := NewContext(NewApp(), Manifest{Name: "weather"}, WithDataDir(root), WithPluginID("plg_bbbbbbbbbbbbbbbbbbbbbbbbbb"))
	dir, err := ctx.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "plugins", "plg_bbbbbbbbbbbbbbbbbbbbbbbbbb")
	if dir != want {
		t.Fatalf("dir = %q, want %q", dir, want)
	}
}

func TestContextDataDirUnavailable(t *testing.T) {
	ctx := NewContext(NewApp(), Manifest{Name: "weather"})
	if _, err := ctx.DataDir(); !errors.Is(err, ErrDataDirUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestContextDataDirRejectsUnsafePluginID(t *testing.T) {
	ctx := NewContext(NewApp(), Manifest{Name: "weather"}, WithPluginID("../weather"), WithDataDir(t.TempDir()))
	if _, err := ctx.DataDir(); err == nil {
		t.Fatal("unsafe plugin id should be rejected")
	}
}

func TestContextGoWhenActionReadyRunsAfterAdapterReady(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter := &sdkStatefulAdapter{started: make(chan struct{})}
	app := NewApp(WithAdapter(adapter))
	task := make(chan error, 1)
	plugin := Define(
		Manifest{Name: "proactive"},
		struct{}{},
		func(ctx *Context, _ struct{}) error {
			ctx.GoWhenActionReady("welcome", func(ctx context.Context) error {
				task <- ctx.Err()
				return nil
			})
			return nil
		},
	)
	if err := InstallDefault(app, plugin); err != nil {
		t.Fatal(err)
	}
	errc := make(chan error, 1)
	go func() {
		errc <- app.runtime.Run(ctx)
	}()
	select {
	case <-adapter.started:
	case <-time.After(time.Second):
		t.Fatal("adapter did not start")
	}
	select {
	case err := <-task:
		t.Fatalf("task ran before adapter was ready: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	adapter.emit(core.AdapterState{Protocol: testProtocol, Kind: core.AdapterStateReady, ActionReady: true})
	select {
	case err := <-task:
		if err != nil {
			t.Fatalf("task context err = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("task did not run after action ready")
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

type sdkTestAdapter struct {
	client ActionClient
}

func (a sdkTestAdapter) Protocol() Protocol                         { return testProtocol }
func (a sdkTestAdapter) Start(context.Context, core.EmitFunc) error { return nil }
func (a sdkTestAdapter) Client() ActionClient                       { return a.client }

type sdkStatefulAdapter struct {
	sink    core.AdapterStateSink
	started chan struct{}
}

func (*sdkStatefulAdapter) Protocol() Protocol { return testProtocol }
func (a *sdkStatefulAdapter) Start(ctx context.Context, _ core.EmitFunc) error {
	close(a.started)
	<-ctx.Done()
	return ctx.Err()
}
func (*sdkStatefulAdapter) Client() ActionClient { return nil }
func (*sdkStatefulAdapter) State() core.AdapterState {
	return core.AdapterState{Protocol: testProtocol, Kind: core.AdapterStateUnknown}
}
func (a *sdkStatefulAdapter) SetStateSink(sink core.AdapterStateSink) {
	a.sink = sink
}
func (a *sdkStatefulAdapter) emit(state core.AdapterState) {
	a.sink(context.Background(), state)
}

type sdkTestClient struct {
	target ReplyTarget
	chain  coremsg.Chain
}

func (c *sdkTestClient) Send(_ context.Context, target ReplyTarget, chain coremsg.Chain) (MessageReceipt, error) {
	c.target = target
	c.chain = chain
	return MessageReceipt{ID: "sent"}, nil
}

type sdkConfigStore struct {
	plugin      string
	assignments []ConfigAssignment
	resets      [][]string
}

func (s *sdkConfigStore) SetPluginConfig(_ context.Context, plugin string, assignments []ConfigAssignment) error {
	s.plugin = plugin
	s.assignments = append([]ConfigAssignment(nil), assignments...)
	return nil
}

func (s *sdkConfigStore) ResetPluginConfig(_ context.Context, _ string, paths [][]string) error {
	s.resets = append([][]string(nil), paths...)
	return nil
}
