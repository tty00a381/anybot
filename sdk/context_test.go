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

func TestContextLoggerScopesPluginName(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	ctx := NewContext(
		core.New(core.WithLogger(logger)),
		Manifest{Name: "weather"},
		WithPluginID("weather"),
	)
	ctx.Logger().Info("ready")
	if !strings.Contains(out.String(), "plugin_id=weather") || !strings.Contains(out.String(), "plugin_name=weather") {
		t.Fatalf("log output = %q", out.String())
	}
}

func TestContextSendTextUsesActionClient(t *testing.T) {
	client := &sdkTestClient{}
	ctx := NewContext(
		core.New(core.WithAdapter(sdkTestAdapter{client: client})),
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
	app := core.New()
	first := NewContext(app, Manifest{Name: "first"}, WithPluginID("first"))
	second := NewContext(app, Manifest{Name: "second"}, WithPluginID("second"))
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
	app := core.New()
	ctx := NewContext(app, Manifest{Name: "ratelimit"}, WithPluginID("ratelimit"))
	if err := ctx.UseGlobal(Timeout(time.Second)); !errors.Is(err, ErrGlobalMiddlewareUnavailable) {
		t.Fatalf("err = %v", err)
	}
	ctx = NewContext(app, Manifest{Name: "ratelimit"}, WithEnvironment(Environment{PluginID: "ratelimit", AllowGlobalMiddleware: true}))
	if err := ctx.UseGlobal(Timeout(time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestContextRouteNamesArePluginScoped(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	app := core.New(core.WithLogger(logger))
	app.Use(core.Trace(logger))
	ctx := NewContext(app, Manifest{Name: "weather"}, WithPluginID("weather"))
	ctx.Command("weather").Name("command").Handle(func(*EventContext) error {
		return nil
	})
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", Text: "/weather"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "route=weather.command") {
		t.Fatalf("route name was not scoped:\n%s", out.String())
	}
}

func TestContextRouteNamesKeepExplicitPluginPrefix(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	app := core.New(core.WithLogger(logger))
	app.Use(core.Trace(logger))
	ctx := NewContext(app, Manifest{Name: "weather"}, WithPluginID("weather"))
	ctx.Command("weather").Name("weather.command").Handle(func(*EventContext) error {
		return nil
	})
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", Text: "/weather"}); err != nil {
		t.Fatal(err)
	}
	log := out.String()
	if strings.Contains(log, "route=weather.weather.command") || !strings.Contains(log, "route=weather.command") {
		t.Fatalf("route name was double-scoped:\n%s", log)
	}
}

func TestContextSessionsArePluginScoped(t *testing.T) {
	app := core.New()
	first := NewContext(app, Manifest{Name: "first"}, WithPluginID("first"))
	second := NewContext(app, Manifest{Name: "second"}, WithPluginID("second"))
	event := core.NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
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
	app := core.New()
	ctx := NewContext(app, Manifest{Name: "天气"}, WithPluginID("weather_12345678"))
	if ctx.PluginID() != "weather_12345678" {
		t.Fatalf("plugin id = %q", ctx.PluginID())
	}
	if key := ctx.SessionBy("settings").Key(); key != "weather_12345678.settings" {
		t.Fatalf("session key = %q", key)
	}
}

func TestTypedStateUsesEventContext(t *testing.T) {
	type profile struct {
		Name string `json:"name"`
	}
	app := core.New()
	ctx := NewContext(app, Manifest{Name: "profile"}, WithPluginID("profile"))
	event := core.NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
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
	app := core.New()
	ctx := NewContext(app, Manifest{Name: "counter"}, WithPluginID("counter"))
	event := core.NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
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

func TestTypedStateUnavailable(t *testing.T) {
	if _, _, err := UserState[int](nil, nil, "count").Load(); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("err = %v", err)
	}
	ctx := NewContext(nil, Manifest{Name: "counter"}, WithPluginID("counter"))
	if err := UserState[int](ctx, nil, "count").Save(1, 0); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("err = %v", err)
	}
	ctx = NewContext(core.New(), Manifest{Name: "counter"}, WithPluginID("counter"))
	if err := UserState[int](ctx, nil, "count").Save(1, 0); !errors.Is(err, ErrEventContextUnavailable) {
		t.Fatalf("err = %v", err)
	}
	app := core.New()
	ctx = NewContext(app, Manifest{Name: "counter"}, WithPluginID("counter"))
	event := core.NewTestContext(app, &core.Event{Protocol: testProtocol, UserID: "42", Type: "message"})
	if _, _, err := UserState[int](ctx, event, "").Load(); !errors.Is(err, ErrStateKeyRequired) {
		t.Fatalf("err = %v", err)
	}
	if err := UserState[int](ctx, event, "   ").Save(1, 0); !errors.Is(err, ErrStateKeyRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestContextConfigWritesThroughStore(t *testing.T) {
	store := &sdkConfigStore{}
	app := core.New()
	ctx := NewContext(app, Manifest{Name: "Minecraft 管理"}, WithPluginID("mc_admin"), WithConfigStore(store))
	if !ctx.Config().Available() {
		t.Fatal("config handle should be available")
	}
	if err := ctx.Config().Set(context.Background(), "bridge.group_to_game", "prefix"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.assignments, []ConfigAssignment{
		{Path: []string{"bridge", "group_to_game"}, Value: "prefix"},
	}) || store.plugin != "mc_admin" {
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
	ctx := NewContext(core.New(), Manifest{Name: "minecraft"}, WithPluginID("minecraft"))
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
	ctx := NewContext(core.New(), Manifest{Name: "天气"}, WithPluginID("weather"), WithDataDir(root))
	dir, err := ctx.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "plugins", "weather")
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
	ctx := NewContext(core.New(), Manifest{Name: "weather"}, WithDataDir(root), WithPluginID("weather_12345678"))
	dir, err := ctx.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "plugins", "weather_12345678")
	if dir != want {
		t.Fatalf("dir = %q, want %q", dir, want)
	}
}

func TestContextDataDirUnavailable(t *testing.T) {
	ctx := NewContext(core.New(), Manifest{Name: "weather"})
	if _, err := ctx.DataDir(); !errors.Is(err, ErrDataDirUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestContextDataDirRejectsUnsafePluginID(t *testing.T) {
	ctx := NewContext(core.New(), Manifest{Name: "weather"}, WithPluginID("../weather"), WithDataDir(t.TempDir()))
	if _, err := ctx.DataDir(); err == nil {
		t.Fatal("unsafe plugin id should be rejected")
	}
}

type sdkTestAdapter struct {
	client ActionClient
}

func (a sdkTestAdapter) Protocol() Protocol                         { return testProtocol }
func (a sdkTestAdapter) Start(context.Context, core.EmitFunc) error { return nil }
func (a sdkTestAdapter) Client() ActionClient                       { return a.client }

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
