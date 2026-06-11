package host

import (
	"github.com/tty00a381/anybot/app/plugins/admin"
	"github.com/tty00a381/anybot/app/plugins/echo"
	"github.com/tty00a381/anybot/app/plugins/help"
	"github.com/tty00a381/anybot/app/plugins/ratelimit"
	absdk "github.com/tty00a381/anybot/sdk"
)

// EmptyRegistry 返回不包含任何已构建外部插件的注册表。
func EmptyRegistry() absdk.Registry {
	return absdk.NewRegistry()
}

// BuiltinPlugin 描述 AnyBot 随框架发布的一种内置插件来源。
type BuiltinPlugin struct {
	Source         string
	Factory        absdk.Factory
	DefaultEnabled bool
}

// BuiltinPlugins 返回 AnyBot 随框架发布的内置插件来源。
func BuiltinPlugins() []BuiltinPlugin {
	return []BuiltinPlugin{
		{Source: "help", Factory: help.Plugin.Factory(), DefaultEnabled: true},
		{Source: "echo", Factory: echo.Plugin.Factory()},
		{Source: "admin", Factory: admin.Plugin.Factory()},
		{Source: "ratelimit", Factory: ratelimit.Plugin.Factory()},
	}
}

// BuiltinFactory 返回指定内置插件来源的工厂。
func BuiltinFactory(source string) (absdk.Factory, bool) {
	for _, plugin := range BuiltinPlugins() {
		if plugin.Source == source {
			return plugin.Factory, true
		}
	}
	return absdk.Factory{}, false
}

// BuiltinDefaultEnabled 返回内置插件在新工作目录中的默认启用状态。
func BuiltinDefaultEnabled(source string) bool {
	for _, plugin := range BuiltinPlugins() {
		if plugin.Source == source {
			return plugin.DefaultEnabled
		}
	}
	return false
}

// RegistryForLock 根据插件安装锁和已构建的外部插件注册表创建运行时注册表。
func RegistryForLock(lock PluginLock, external absdk.Registry) (absdk.Registry, error) {
	lock.applyDefaults()
	registry := absdk.NewRegistry()
	for _, item := range lock.Plugins {
		switch {
		case item.Builtin != "":
			factory, ok := BuiltinFactory(item.Builtin)
			if !ok {
				return absdk.Registry{}, NewUnknownBuiltinError(item.Builtin)
			}
			if err := registry.Register(factory.WithPluginID(item.ID)); err != nil {
				return absdk.Registry{}, err
			}
		case item.Module != "":
			factory, ok := external.Factory(item.ID)
			if !ok {
				continue
			}
			if err := registry.Register(factory.WithPluginID(item.ID)); err != nil {
				return absdk.Registry{}, err
			}
		}
	}
	return registry, nil
}

func pluginAllowsGlobalMiddleware(source string) bool {
	return source == "ratelimit"
}
