package sdk

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tty00a381/anybot/core"
	"gopkg.in/yaml.v3"
)

func TestDefinitionFactoryBuildsConfiguredPlugin(t *testing.T) {
	type config struct {
		Command string `yaml:"command"`
	}
	var called bool
	definition := Define(Manifest{Name: "hello", Version: "1.0.0"}, config{Command: "hello"}, func(ctx *Context, cfg config) error {
		ctx.Command(cfg.Command).Handle(func(*EventContext) error {
			called = true
			return nil
		})
		return nil
	})
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("command: hi\n"), &node); err != nil {
		t.Fatal(err)
	}
	plugin, err := definition.Factory().Build(node)
	if err != nil {
		t.Fatal(err)
	}
	app := core.New()
	if err := Install(app, plugin); err != nil {
		t.Fatal(err)
	}
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", Text: "/hi"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("configured command did not run")
	}
}

func TestDefinitionIsNotInstallablePlugin(t *testing.T) {
	definition := Define(Manifest{Name: "hello"}, struct{}{}, func(*Context, struct{}) error {
		return nil
	})
	if _, ok := any(definition).(Plugin); ok {
		t.Fatal("plugin definition should be built before installation")
	}
}

func TestDefinitionValidatesConfig(t *testing.T) {
	definition := Define(Manifest{Name: "strict"}, validatingConfig{}, func(*Context, validatingConfig) error {
		return nil
	})
	_, err := definition.Factory().Build(yaml.Node{})
	if err == nil || err.Error() != "not ok" {
		t.Fatalf("err = %v", err)
	}
}

func TestDefinitionFactoryRejectsInvalidPluginName(t *testing.T) {
	definition := Define(Manifest{Name: "../bad"}, struct{}{}, func(*Context, struct{}) error {
		return nil
	})
	_, err := definition.Factory().Build(yaml.Node{})
	if err == nil || !strings.Contains(err.Error(), `plugin name "../bad" is invalid`) {
		t.Fatalf("err = %v", err)
	}
}

func TestRegistryRejectsInvalidPluginName(t *testing.T) {
	registry := NewRegistry()
	definition := Define(Manifest{Name: "ok"}, struct{}{}, func(*Context, struct{}) error {
		return nil
	})
	if err := registry.Register(definition.Factory().WithName("bad/name")); err == nil ||
		!strings.Contains(err.Error(), `plugin name "bad/name" is invalid`) {
		t.Fatalf("err = %v", err)
	}
}

func TestDefinitionClonesDefaultConfig(t *testing.T) {
	type config struct {
		Tags []string `yaml:"tags"`
	}
	var seen []string
	definition := Define(Manifest{Name: "clone"}, config{Tags: []string{"default"}}, func(_ *Context, cfg config) error {
		seen = append(seen, cfg.Tags[0])
		cfg.Tags[0] = "mutated"
		return nil
	})
	for i := 0; i < 2; i++ {
		plugin, err := definition.Factory().Build(yaml.Node{})
		if err != nil {
			t.Fatal(err)
		}
		if err := Install(core.New(), plugin); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 2 || seen[0] != "default" || seen[1] != "default" {
		t.Fatalf("seen = %#v", seen)
	}
}

func TestFactoryWithNameAliasesPluginContext(t *testing.T) {
	type config struct {
		Command string `yaml:"command"`
	}
	var contextName string
	definition := Define(Manifest{Name: "weather", Version: "1.0.0"}, config{Command: "weather"}, func(ctx *Context, cfg config) error {
		contextName = ctx.Manifest().Name
		return nil
	})
	factory := definition.Factory().WithName("daily_weather")
	if factory.Info.Name != "daily_weather" || factory.Info.Version != "1.0.0" {
		t.Fatalf("factory info = %#v", factory.Info)
	}
	plugin, err := factory.Build(yaml.Node{})
	if err != nil {
		t.Fatal(err)
	}
	if manifest := plugin.Manifest(); manifest.Name != "daily_weather" || manifest.Version != "1.0.0" {
		t.Fatalf("plugin manifest = %#v", manifest)
	}
	app := core.New()
	if err := Install(app, plugin); err != nil {
		t.Fatal(err)
	}
	if contextName != "daily_weather" {
		t.Fatalf("context manifest name = %q", contextName)
	}
}

func TestDefinitionFactoryResolvesEnvConfig(t *testing.T) {
	type config struct {
		Token string `yaml:"token"`
	}
	t.Setenv("ANYBOT_TEST_TOKEN", "secret-token")
	var seen string
	definition := Define(Manifest{Name: "secret"}, config{}, func(_ *Context, cfg config) error {
		seen = cfg.Token
		return nil
	})
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("token: !env ANYBOT_TEST_TOKEN\n"), &node); err != nil {
		t.Fatal(err)
	}
	plugin, err := definition.Factory().Build(node)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(core.New(), plugin); err != nil {
		t.Fatal(err)
	}
	if seen != "secret-token" {
		t.Fatalf("token = %q", seen)
	}
}

func TestDefinitionFactoryRejectsMissingEnvConfig(t *testing.T) {
	type config struct {
		Token string `yaml:"token"`
	}
	definition := Define(Manifest{Name: "secret"}, config{}, func(*Context, config) error {
		return nil
	})
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("token: !env ANYBOT_MISSING_TOKEN\n"), &node); err != nil {
		t.Fatal(err)
	}
	_, err := definition.Factory().Build(node)
	if err == nil || err.Error() != "plugin config environment variable ANYBOT_MISSING_TOKEN is not set" {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallWrapsSetupError(t *testing.T) {
	definition := Define(Manifest{Name: "broken"}, struct{}{}, func(*Context, struct{}) error {
		return errors.New("boom")
	})
	plugin, buildErr := definition.Build()
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	err := Install(core.New(), plugin)
	if err == nil || !strings.Contains(err.Error(), "broken") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallRejectsNilApp(t *testing.T) {
	if err := Install(nil); err == nil || err.Error() != "anybot: app is nil" {
		t.Fatalf("err = %v", err)
	}
}

type validatingConfig struct {
	OK bool `yaml:"ok"`
}

func (cfg validatingConfig) Validate() error {
	if !cfg.OK {
		return errors.New("not ok")
	}
	return nil
}
