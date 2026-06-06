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
	app, err := NewApp(cfg, DefaultRegistry(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	plugins := app.Plugins()
	if len(plugins) != 1 || plugins[0].Name != "help" {
		t.Fatalf("plugins = %#v", plugins)
	}
	names, err := EnabledPlugins(cfg, DefaultRegistry())
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
	app, err := NewApp(cfg, DefaultRegistry(), slog.Default())
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
