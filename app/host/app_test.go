package host

import (
	"errors"
	"log/slog"
	"reflect"
	"testing"

	"github.com/tty00a381/anybot/adapters/onebot11"
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
