package sdk

import (
	"context"
	"errors"
	"os"
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
	definition := Define(Spec[config]{
		Manifest:      Manifest{Name: "hello", Version: "1.0.0"},
		DefaultConfig: config{Command: "hello"},
		Setup: func(ctx *Context, cfg config) error {
			ctx.Command(cfg.Command).Handle(func(*EventContext) error {
				called = true
				return nil
			})
			return nil
		},
	})
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("command: hi\n"), &node); err != nil {
		t.Fatal(err)
	}
	plugin, err := definition.Factory().Build(node)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
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

func TestDefinitionBuildWithUsesTypedConfig(t *testing.T) {
	type config struct {
		Command string `yaml:"command"`
	}
	var called bool
	definition := Define(Spec[config]{
		Manifest:      Manifest{Name: "hello", Version: "1.0.0"},
		DefaultConfig: config{Command: "hello"},
		Setup: func(ctx *Context, cfg config) error {
			ctx.Command(cfg.Command).Handle(func(*EventContext) error {
				called = true
				return nil
			})
			return nil
		},
	})
	plugin, err := definition.BuildWith(config{Command: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
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
	definition := Define(Spec[struct{}]{
		Manifest:      Manifest{Name: "hello"},
		DefaultConfig: struct{}{},
		Setup: func(*Context, struct{}) error {
			return nil
		},
	})
	if _, ok := any(definition).(Plugin); ok {
		t.Fatal("plugin definition should be built before installation")
	}
}

func TestDefinitionValidatesConfig(t *testing.T) {
	definition := Define(Spec[validatingConfig]{
		Manifest:      Manifest{Name: "strict"},
		DefaultConfig: validatingConfig{},
		Setup: func(*Context, validatingConfig) error {
			return nil
		},
	})
	_, err := definition.Factory().Build(yaml.Node{})
	if err == nil || err.Error() != "not ok" {
		t.Fatalf("err = %v", err)
	}
}

func TestManifestNameIsDisplayOnly(t *testing.T) {
	var seen string
	definition := Define(Spec[struct{}]{
		Manifest:      Manifest{Name: "天气/每日", Version: "1.0.0"},
		DefaultConfig: struct{}{},
		Setup: func(ctx *Context, _ struct{}) error {
			seen = ctx.Manifest().Name
			return nil
		},
	})
	plugin, err := definition.Factory().Build(yaml.Node{})
	if err != nil {
		t.Fatal(err)
	}
	if err := InstallWith(NewApp(), Environment{PluginID: testPluginID}, plugin); err != nil {
		t.Fatal(err)
	}
	if seen != "天气/每日" {
		t.Fatalf("manifest name = %q", seen)
	}
}

func TestValidatePluginIDRequiresGeneratedIdentifier(t *testing.T) {
	for _, id := range []string{testPluginID, "plg_bbbbbbbbbbbbbbbbbbbbbbbbbb"} {
		if err := ValidatePluginID(id); err != nil {
			t.Fatalf("ValidatePluginID(%q) = %v", id, err)
		}
	}
	for _, id := range []string{"weather", "anybot_weather", "weather2", "plg_abc123", "Weather", "weather-bot", "2weather", "天气", "weather.bot", "weather bot", "../weather"} {
		if err := ValidatePluginID(id); err == nil {
			t.Fatalf("ValidatePluginID(%q) should fail", id)
		}
	}
}

func TestRegistryRejectsInvalidPluginID(t *testing.T) {
	registry := NewRegistry()
	definition := Define(Spec[struct{}]{
		Manifest:      Manifest{Name: "ok"},
		DefaultConfig: struct{}{},
		Setup: func(*Context, struct{}) error {
			return nil
		},
	})
	if err := registry.Register(definition.Factory().WithPluginID("bad/name")); err == nil ||
		!strings.Contains(err.Error(), `plugin id "bad/name" is invalid`) {
		t.Fatalf("err = %v", err)
	}
}

func TestDefinitionClonesDefaultConfig(t *testing.T) {
	type config struct {
		Tags []string `yaml:"tags"`
	}
	var seen []string
	definition := Define(Spec[config]{
		Manifest:      Manifest{Name: "clone"},
		DefaultConfig: config{Tags: []string{"default"}},
		Setup: func(_ *Context, cfg config) error {
			seen = append(seen, cfg.Tags[0])
			cfg.Tags[0] = "mutated"
			return nil
		},
	})
	for i := 0; i < 2; i++ {
		plugin, err := definition.Factory().Build(yaml.Node{})
		if err != nil {
			t.Fatal(err)
		}
		if err := Install(NewApp(), plugin); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 2 || seen[0] != "default" || seen[1] != "default" {
		t.Fatalf("seen = %#v", seen)
	}
}

func TestDefinitionBuildWithClonesConfig(t *testing.T) {
	type config struct {
		Tags []string `yaml:"tags"`
	}
	input := config{Tags: []string{"custom"}}
	var seen string
	definition := Define(Spec[config]{
		Manifest:      Manifest{Name: "clone"},
		DefaultConfig: config{Tags: []string{"default"}},
		Setup: func(_ *Context, cfg config) error {
			seen = cfg.Tags[0]
			cfg.Tags[0] = "mutated"
			return nil
		},
	})
	plugin, err := definition.BuildWith(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(NewApp(), plugin); err != nil {
		t.Fatal(err)
	}
	if seen != "custom" || input.Tags[0] != "custom" {
		t.Fatalf("seen=%q input=%#v", seen, input)
	}
}

func TestFactoryWithPluginIDLeavesManifestUntouched(t *testing.T) {
	type config struct {
		Command string `yaml:"command"`
	}
	var contextName, contextID string
	definition := Define(Spec[config]{
		Manifest:      Manifest{Name: "weather", Version: "1.0.0"},
		DefaultConfig: config{Command: "weather"},
		Setup: func(ctx *Context, cfg config) error {
			contextName = ctx.Manifest().Name
			contextID = ctx.PluginID()
			return nil
		},
	})
	factory := definition.Factory().WithPluginID(testPluginID)
	if factory.Info.Name != "weather" || factory.PluginID != testPluginID || factory.Info.Version != "1.0.0" {
		t.Fatalf("factory info = %#v", factory.Info)
	}
	plugin, err := factory.Build(yaml.Node{})
	if err != nil {
		t.Fatal(err)
	}
	if manifest := plugin.Manifest(); manifest.Name != "weather" || manifest.Version != "1.0.0" {
		t.Fatalf("plugin manifest = %#v", manifest)
	}
	app := NewApp()
	if err := InstallWith(app, Environment{PluginID: factory.PluginID}, plugin); err != nil {
		t.Fatal(err)
	}
	if contextName != "weather" || contextID != testPluginID {
		t.Fatalf("context manifest=%q id=%q", contextName, contextID)
	}
}

const testPluginID = "plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestDefinitionFactoryResolvesEnvConfig(t *testing.T) {
	type config struct {
		Token string `yaml:"token"`
	}
	t.Setenv("ANYBOT_TEST_TOKEN", "secret-token")
	var seen string
	definition := Define(Spec[config]{
		Manifest:      Manifest{Name: "secret"},
		DefaultConfig: config{},
		Setup: func(_ *Context, cfg config) error {
			seen = cfg.Token
			return nil
		},
	})
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("token: !env ANYBOT_TEST_TOKEN\n"), &node); err != nil {
		t.Fatal(err)
	}
	plugin, err := definition.Factory().Build(node)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(NewApp(), plugin); err != nil {
		t.Fatal(err)
	}
	if seen != "secret-token" {
		t.Fatalf("token = %q", seen)
	}
}

func TestDefinitionFactoryResolvesMissingEnvConfigAsEmpty(t *testing.T) {
	type config struct {
		Token string `yaml:"token"`
	}
	envName := "ANYBOT_MISSING_TOKEN"
	old, ok := os.LookupEnv(envName)
	t.Cleanup(func() {
		if ok {
			if err := os.Setenv(envName, old); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Unsetenv(envName); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err := os.Unsetenv(envName); err != nil {
		t.Fatal(err)
	}
	var seen string
	definition := Define(Spec[config]{
		Manifest:      Manifest{Name: "secret"},
		DefaultConfig: config{},
		Setup: func(_ *Context, cfg config) error {
			seen = cfg.Token
			return nil
		},
	})
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("token: !env ANYBOT_MISSING_TOKEN\n"), &node); err != nil {
		t.Fatal(err)
	}
	plugin, err := definition.Factory().Build(node)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(NewApp(), plugin); err != nil {
		t.Fatal(err)
	}
	if seen != "" {
		t.Fatalf("token = %q", seen)
	}
}

func TestInstallWrapsSetupError(t *testing.T) {
	definition := Define(Spec[struct{}]{
		Manifest:      Manifest{Name: "broken"},
		DefaultConfig: struct{}{},
		Setup: func(*Context, struct{}) error {
			return errors.New("boom")
		},
	})
	plugin, buildErr := definition.Build()
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	err := Install(NewApp(), plugin)
	if err == nil || !strings.Contains(err.Error(), "broken") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallRejectsMissingSetup(t *testing.T) {
	definition := Define(Spec[struct{}]{
		Manifest:      Manifest{Name: "empty"},
		DefaultConfig: struct{}{},
	})
	plugin, buildErr := definition.Build()
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	err := Install(NewApp(), plugin)
	if err == nil ||
		!strings.Contains(err.Error(), "安装插件 empty 失败") ||
		!strings.Contains(err.Error(), "plugin empty setup function is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallRejectsNilApp(t *testing.T) {
	if err := Install(nil); err == nil || err.Error() != "anybot: app is nil" {
		t.Fatalf("err = %v", err)
	}
	if err := InstallDefault(nil); err == nil || err.Error() != "anybot: app is nil" {
		t.Fatalf("err = %v", err)
	}
	if err := InstallDefaultWith(nil, Environment{}, nil); err == nil || err.Error() != "anybot: app is nil" {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallRejectsNilPluginInputs(t *testing.T) {
	app := NewApp()
	if err := Install(app, nil); err == nil || err.Error() != "anybot: plugin is nil" {
		t.Fatalf("err = %v", err)
	}
	if err := InstallWith(app, Environment{}, nil); err == nil || err.Error() != "anybot: plugin is nil" {
		t.Fatalf("err = %v", err)
	}
	if err := InstallDefault(app, nil); err == nil || err.Error() != "anybot: plugin definition is nil" {
		t.Fatalf("err = %v", err)
	}
	if err := InstallDefaultWith(app, Environment{}, nil); err == nil || err.Error() != "anybot: plugin definition is nil" {
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
