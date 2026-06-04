package anybot

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestUsePluginsOrderAndManifest(t *testing.T) {
	app := New()
	var calls []string
	err := app.UsePlugins(
		PluginFunc{
			Info: Manifest{Name: "first", Version: "1.0.0"},
			Fn: func(*App) error {
				calls = append(calls, "first")
				return nil
			},
		},
		PluginFunc{
			Info: Manifest{Name: "second", Version: "1.0.0"},
			Fn: func(*App) error {
				calls = append(calls, "second")
				return nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("calls=%#v", calls)
	}
	plugins := app.Plugins()
	if len(plugins) != 2 || plugins[0].Name != "first" || plugins[1].Name != "second" {
		t.Fatalf("plugins=%#v", plugins)
	}
}

func TestUsePluginWrapsInstallError(t *testing.T) {
	app := New()
	err := app.UsePlugin(PluginFunc{
		Info: Manifest{Name: "broken"},
		Fn: func(*App) error {
			return errors.New("boom")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "broken") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err=%v", err)
	}
}
