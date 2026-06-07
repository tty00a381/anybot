package host

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tty00a381/anybot/adapters/onebot11"
	absdk "github.com/tty00a381/anybot/sdk"
)

func TestNewAppInstallsConfiguredPlugins(t *testing.T) {
	enabled := true
	disabled := false
	var called bool
	registry := absdk.NewRegistry()
	help := absdk.Define(absdk.Manifest{Name: "help"}, struct{}{}, func(ctx *absdk.Context, _ struct{}) error {
		ctx.Command("help").Handle(func(*absdk.EventContext) error {
			called = true
			return nil
		})
		return nil
	})
	echo := absdk.Define(absdk.Manifest{Name: "echo"}, struct{}{}, func(*absdk.Context, struct{}) error {
		t.Fatal("disabled plugin should not be installed")
		return nil
	})
	if err := registry.Register(help.Factory()); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(echo.Factory()); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Adapter: AdapterConfig{
			Protocol:  "onebot11",
			Transport: onebotTransport("reverse_ws", "127.0.0.1:0"),
		},
		Plugins: map[string]PluginEntry{
			"help": {Enabled: &enabled},
			"echo": {Enabled: &disabled},
		},
	}
	app, err := NewApp(cfg, registry, slog.Default(), WithRuntimeState())
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Dispatch(context.Background(), &absdk.Event{Type: "message", Text: "/help"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("enabled plugin route did not run")
	}
	names, err := EnabledPlugins(cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"help"}) {
		t.Fatalf("enabled = %#v", names)
	}
}

func TestNewAppInjectsSuperUsers(t *testing.T) {
	enabled := true
	cfg := Config{
		Adapter: AdapterConfig{
			Protocol:  "onebot11",
			Transport: onebotTransport("reverse_ws", "127.0.0.1:0"),
		},
		Security: SecurityConfig{SuperUsers: []string{"42"}},
		Plugins: map[string]PluginEntry{
			"admin": {Enabled: &enabled},
		},
	}
	app, err := NewApp(cfg, DefaultRegistry(), slog.Default(), WithRuntimeState())
	if err != nil {
		t.Fatal(err)
	}
	if !app.IsSuperUser("42") || app.IsSuperUser("7") {
		t.Fatalf("superusers = %#v", app.SuperUsers())
	}
}

func TestNewAppInjectsPluginConfigStore(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugin_config_dir: plugins.d
plugins: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(dir, "plugins.d", "minecraft.yaml")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, []byte("enabled: true\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	registry := absdk.NewRegistry()
	module := absdk.Define(absdk.Manifest{Name: "minecraft"}, struct{}{}, func(ctx *absdk.Context, _ struct{}) error {
		return ctx.Config().Set(context.Background(), "bridge.group_to_game", "prefix")
	})
	if err := registry.Register(module.Factory()); err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp(cfg, registry, slog.Default(), WithConfigPath(configPath)); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, pluginPath)
	if !strings.Contains(out, "bridge:") || !strings.Contains(out, "group_to_game: prefix") {
		t.Fatalf("plugin config:\n%s", out)
	}
}

func TestNewAppUsesLoadedConfigPathForPluginConfigStore(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugin_config_dir: plugins.d
plugins: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(dir, "plugins.d", "memory.yaml")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, []byte("enabled: true\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	registry := absdk.NewRegistry()
	module := absdk.Define(absdk.Manifest{Name: "memory"}, struct{}{}, func(ctx *absdk.Context, _ struct{}) error {
		return ctx.Config().Set(context.Background(), "state.path", "memory.db")
	})
	if err := registry.Register(module.Factory()); err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp(cfg, registry, slog.Default()); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, pluginPath)
	if !strings.Contains(out, "state:") || !strings.Contains(out, "path: memory.db") {
		t.Fatalf("plugin config:\n%s", out)
	}
}

func TestNewAppUsesPersistentStoreFromConfigPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(cfg, DefaultRegistry(), slog.Default(), WithRuntimeState())
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Set(context.Background(), "binding", []byte("uuid"), 0); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(dir, ".anybot", "store.json")
	if _, err := os.Stat(storePath); err != nil {
		t.Fatal(err)
	}
	next, err := NewApp(cfg, DefaultRegistry(), slog.Default(), WithRuntimeState())
	if err != nil {
		t.Fatal(err)
	}
	data, ok, err := next.Store().Get(context.Background(), "binding")
	if err != nil || !ok || string(data) != "uuid" {
		t.Fatalf("ok=%v err=%v data=%q", ok, err, data)
	}
}

func TestNewAppCanDisablePersistentStore(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`runtime:
  store:
    type: memory
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(cfg, DefaultRegistry(), slog.Default(), WithRuntimeState())
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Set(context.Background(), "binding", []byte("uuid"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".anybot", "store.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("store file should not exist: %v", err)
	}
}

func TestNewAppDoesNotUseRuntimeStateByDefault(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(cfg, DefaultRegistry(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Set(context.Background(), "binding", []byte("uuid"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".anybot", "store.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("store file should not exist: %v", err)
	}
}

func TestNewAppRejectsEscapingStorePath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`runtime:
  store:
    type: file
    path: ../store.json
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp(cfg, DefaultRegistry(), slog.Default(), WithRuntimeState()); err == nil || !strings.Contains(err.Error(), "runtime.store.path") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewAppTrimsRuntimeDataDirForStorePath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`runtime:
  data_dir: " state "
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(cfg, DefaultRegistry(), slog.Default(), WithRuntimeState())
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Set(context.Background(), "binding", []byte("uuid"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "state", "store.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, " state ", "store.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("untrimmed store file should not exist: %v", err)
	}
}

func TestValidateConfigDoesNotInstallPlugins(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins:
  memory:
    enabled: true
    config: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var installed bool
	registry := absdk.NewRegistry()
	module := absdk.Define(absdk.Manifest{Name: "memory"}, struct{}{}, func(*absdk.Context, struct{}) error {
		installed = true
		return nil
	})
	if err := registry.Register(module.Factory()); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(cfg, registry); err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Fatal("ValidateConfig should not execute plugin setup")
	}
	if _, err := os.Stat(filepath.Join(dir, ".anybot", "store.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("store file should not exist: %v", err)
	}
}

func TestValidateConfigRejectsInvalidRuntimeState(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`runtime:
  store:
    type: file
    path: ../store.json
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(cfg, DefaultRegistry()); err == nil || !strings.Contains(err.Error(), "runtime.store.path") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewAppInjectsPluginDataDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`runtime:
  data_dir: state
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins:
  memory:
    enabled: true
    config: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var dataDir string
	registry := absdk.NewRegistry()
	module := absdk.Define(absdk.Manifest{Name: "memory"}, struct{}{}, func(ctx *absdk.Context, _ struct{}) error {
		var err error
		dataDir, err = ctx.DataDir()
		return err
	})
	if err := registry.Register(module.Factory()); err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp(cfg, registry, slog.Default(), WithRuntimeState()); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "state", "plugins", "memory")
	if dataDir != want {
		t.Fatalf("dataDir = %q, want %q", dataDir, want)
	}
	if info, err := os.Stat(dataDir); err != nil || !info.IsDir() {
		t.Fatalf("data dir stat: info=%#v err=%v", info, err)
	}
}

func TestNewAppUsesPluginAliasForDataDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(configPath, []byte(`adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:0"
plugins:
  mc_admin:
    enabled: true
    config: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var dataDir string
	registry := absdk.NewRegistry()
	module := absdk.Define(absdk.Manifest{Name: "minecraft"}, struct{}{}, func(ctx *absdk.Context, _ struct{}) error {
		var err error
		dataDir, err = ctx.DataDir()
		return err
	})
	if err := registry.Register(module.Factory().WithName("mc_admin")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp(cfg, registry, slog.Default(), WithRuntimeState()); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".anybot", "plugins", "mc_admin")
	if dataDir != want {
		t.Fatalf("dataDir = %q, want %q", dataDir, want)
	}
}

func TestNewAppRejectsUnknownPlugin(t *testing.T) {
	cfg := Config{
		Adapter: AdapterConfig{
			Protocol:  "onebot11",
			Transport: onebotTransport("reverse_ws", "127.0.0.1:0"),
		},
		Plugins: map[string]PluginEntry{"missing": {}},
	}
	if _, err := NewApp(cfg, DefaultRegistry(), slog.Default()); err == nil {
		t.Fatal("unknown plugin should be rejected")
	} else {
		var unknown UnknownPluginError
		if !errors.As(err, &unknown) || unknown.Name != "missing" {
			t.Fatalf("err = %#v", err)
		}
	}
}

func onebotTransport(kind, listen string) onebot11.TransportConfig {
	return onebot11.TransportConfig{Type: kind, Listen: listen}
}
