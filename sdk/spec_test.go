package sdk

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tty00a381/anybot/core"
	"gopkg.in/yaml.v3"
)

func TestSpecFactoryBuildsConfiguredModule(t *testing.T) {
	type config struct {
		Command string `yaml:"command"`
	}
	var called bool
	spec := Define(Manifest{Name: "hello", Version: "1.0.0"}, config{Command: "hello"}, func(ctx *Context, cfg config) error {
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
	module, err := spec.Factory().Build(node)
	if err != nil {
		t.Fatal(err)
	}
	app := core.New()
	if err := Install(app, module); err != nil {
		t.Fatal(err)
	}
	if err := app.Dispatch(context.Background(), &core.Event{Type: "message", Text: "/hi"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("configured command did not run")
	}
}

func TestSpecValidatesConfig(t *testing.T) {
	spec := Define(Manifest{Name: "strict"}, validatingConfig{}, func(*Context, validatingConfig) error {
		return nil
	})
	_, err := spec.Factory().Build(yaml.Node{})
	if err == nil || err.Error() != "not ok" {
		t.Fatalf("err = %v", err)
	}
}

func TestSpecClonesDefaultConfig(t *testing.T) {
	type config struct {
		Tags []string `yaml:"tags"`
	}
	var seen []string
	spec := Define(Manifest{Name: "clone"}, config{Tags: []string{"default"}}, func(_ *Context, cfg config) error {
		seen = append(seen, cfg.Tags[0])
		cfg.Tags[0] = "mutated"
		return nil
	})
	for i := 0; i < 2; i++ {
		module, err := spec.Factory().Build(yaml.Node{})
		if err != nil {
			t.Fatal(err)
		}
		if err := Install(core.New(), module); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 2 || seen[0] != "default" || seen[1] != "default" {
		t.Fatalf("seen = %#v", seen)
	}
}

func TestFactoryWithNameAliasesModuleContext(t *testing.T) {
	type config struct {
		Command string `yaml:"command"`
	}
	var contextName string
	spec := Define(Manifest{Name: "weather", Version: "1.0.0"}, config{Command: "weather"}, func(ctx *Context, cfg config) error {
		contextName = ctx.Manifest().Name
		return nil
	})
	factory := spec.Factory().WithName("daily_weather")
	if factory.Info.Name != "daily_weather" || factory.Info.Version != "1.0.0" {
		t.Fatalf("factory info = %#v", factory.Info)
	}
	module, err := factory.Build(yaml.Node{})
	if err != nil {
		t.Fatal(err)
	}
	if manifest := module.Manifest(); manifest.Name != "daily_weather" || manifest.Version != "1.0.0" {
		t.Fatalf("module manifest = %#v", manifest)
	}
	app := core.New()
	if err := Install(app, module); err != nil {
		t.Fatal(err)
	}
	if contextName != "daily_weather" {
		t.Fatalf("context manifest name = %q", contextName)
	}
}

func TestSpecFactoryResolvesEnvConfig(t *testing.T) {
	type config struct {
		Token string `yaml:"token"`
	}
	t.Setenv("ANYBOT_TEST_TOKEN", "secret-token")
	var seen string
	spec := Define(Manifest{Name: "secret"}, config{}, func(_ *Context, cfg config) error {
		seen = cfg.Token
		return nil
	})
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("token: !env ANYBOT_TEST_TOKEN\n"), &node); err != nil {
		t.Fatal(err)
	}
	module, err := spec.Factory().Build(node)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(core.New(), module); err != nil {
		t.Fatal(err)
	}
	if seen != "secret-token" {
		t.Fatalf("token = %q", seen)
	}
}

func TestSpecFactoryRejectsMissingEnvConfig(t *testing.T) {
	type config struct {
		Token string `yaml:"token"`
	}
	spec := Define(Manifest{Name: "secret"}, config{}, func(*Context, config) error {
		return nil
	})
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("token: !env ANYBOT_MISSING_TOKEN\n"), &node); err != nil {
		t.Fatal(err)
	}
	_, err := spec.Factory().Build(node)
	if err == nil || err.Error() != "plugin config environment variable ANYBOT_MISSING_TOKEN is not set" {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallWrapsSetupError(t *testing.T) {
	module := Define(Manifest{Name: "broken"}, struct{}{}, func(*Context, struct{}) error {
		return errors.New("boom")
	})
	err := Install(core.New(), module)
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
