package host

import absdk "github.com/tty00a381/anybot/sdk"

const (
	testHelpID      = "plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"
	testEchoID      = "plg_bbbbbbbbbbbbbbbbbbbbbbbbbb"
	testAdminID     = "plg_cccccccccccccccccccccccccc"
	testRateLimitID = "plg_dddddddddddddddddddddddddd"
	testWeatherID   = "plg_eeeeeeeeeeeeeeeeeeeeeeeeee"
	testMemoryID    = "plg_ffffffffffffffffffffffffff"
	testGhostID     = "plg_22222222222222222222222222"
)

func builtinLock(items ...PluginInstall) PluginLock {
	return PluginLock{Plugins: items}
}

func builtinInstall(id, source string) PluginInstall {
	return PluginInstall{ID: id, Builtin: source}
}

func externalInstall(id, module string) PluginInstall {
	return PluginInstall{ID: id, Module: module}
}

func testBuiltinRegistry(t testingT, items ...PluginInstall) absdk.Registry {
	t.Helper()
	registry, err := RegistryForLock(builtinLock(items...), EmptyRegistry())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func testRegistry(t testingT, factories ...absdk.Factory) absdk.Registry {
	t.Helper()
	registry := absdk.NewRegistry()
	for _, factory := range factories {
		if err := registry.Register(factory); err != nil {
			t.Fatal(err)
		}
	}
	return registry
}

type testingT interface {
	Helper()
	Fatal(args ...any)
}
