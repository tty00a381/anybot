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
	)
	ctx.Logger().Info("ready")
	if !strings.Contains(out.String(), "plugin=weather") {
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
	first := NewContext(app, Manifest{Name: "first"})
	second := NewContext(app, Manifest{Name: "second"})
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

func TestContextSessionsArePluginScoped(t *testing.T) {
	app := core.New()
	first := NewContext(app, Manifest{Name: "first"})
	second := NewContext(app, Manifest{Name: "second"})
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

func TestContextConfigWritesThroughStore(t *testing.T) {
	store := &sdkConfigStore{}
	app := core.New(WithConfigStore(store))
	ctx := NewContext(app, Manifest{Name: "minecraft"})
	if !ctx.Config().Available() {
		t.Fatal("config handle should be available")
	}
	if err := ctx.Config().Set(context.Background(), "bridge.group_to_game", "prefix"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.assignments, []ConfigAssignment{
		{Path: []string{"bridge", "group_to_game"}, Value: "prefix"},
	}) || store.plugin != "minecraft" {
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
	ctx := NewContext(core.New(), Manifest{Name: "minecraft"})
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
	ctx := NewContext(core.New(WithDataDir(root)), Manifest{Name: "weather"})
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

func TestContextDataDirUnavailable(t *testing.T) {
	ctx := NewContext(core.New(), Manifest{Name: "weather"})
	if _, err := ctx.DataDir(); !errors.Is(err, ErrDataDirUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestContextDataDirRejectsUnsafePluginName(t *testing.T) {
	ctx := NewContext(core.New(WithDataDir(t.TempDir())), Manifest{Name: "../weather"})
	if _, err := ctx.DataDir(); err == nil {
		t.Fatal("unsafe plugin name should be rejected")
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
