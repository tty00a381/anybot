package host

import (
	"github.com/tty00a381/anybot/app/plugins/admin"
	"github.com/tty00a381/anybot/app/plugins/echo"
	"github.com/tty00a381/anybot/app/plugins/help"
	"github.com/tty00a381/anybot/app/plugins/ratelimit"
	absdk "github.com/tty00a381/anybot/sdk"
)

// DefaultRegistry 返回 anybot 首发内置插件注册表。
func DefaultRegistry() absdk.Registry {
	registry := absdk.NewRegistry()
	mustRegister(registry, help.Plugin.Factory())
	mustRegister(registry, echo.Plugin.Factory())
	mustRegister(registry, admin.Plugin.Factory())
	mustRegister(registry, ratelimit.Plugin.Factory())
	return registry
}

func mustRegister(registry absdk.Registry, factory absdk.Factory) {
	if err := registry.Register(factory); err != nil {
		panic(err)
	}
}

func pluginAllowsGlobalMiddleware(name string) bool {
	return name == "ratelimit"
}
