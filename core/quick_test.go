package core

import (
	"context"
	"errors"
	"testing"
)

func TestDefaultPluginAndLifecycleHelpers(t *testing.T) {
	ResetDefault()
	defer ResetDefault()

	var installed bool
	if err := UsePlugins(PluginFunc{
		Info: Manifest{Name: "quick"},
		Fn: func(*App) error {
			installed = true
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if !installed || len(Default().Plugins()) != 1 {
		t.Fatalf("installed=%v plugins=%#v", installed, Default().Plugins())
	}

	OnStart(func(context.Context) error { return nil })
	OnReady(func(context.Context) error { return nil })
	OnShutdown(func(context.Context) error { return nil })
	OnError(func(*Context, error) {})
	if len(Default().startHooks) != 1 || len(Default().readyHooks) != 1 || len(Default().shutdownHooks) != 1 || len(Default().errorHandlers) != 1 {
		t.Fatalf("hooks start=%d ready=%d shutdown=%d error=%d", len(Default().startHooks), len(Default().readyHooks), len(Default().shutdownHooks), len(Default().errorHandlers))
	}
}

func TestDefaultOnErrorHelperHandlesRouteError(t *testing.T) {
	ResetDefault()
	defer ResetDefault()

	var got error
	OnError(func(_ *Context, err error) {
		got = err
	})
	Command("boom").Handle(func(*Context) error {
		return errors.New("bad")
	})
	_ = Default().dispatch(context.Background(), &Event{Type: "message", Text: "/boom"})
	if got == nil || got.Error() != "bad" {
		t.Fatalf("got=%v", got)
	}
}
