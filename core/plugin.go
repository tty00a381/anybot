package core

import (
	"context"
	"fmt"
)

// Manifest 描述插件的名称、版本、说明和可选配置。
type Manifest struct {
	Name        string
	Version     string
	Description string
	Config      any
}

// Plugin 是可显式安装到 App 的扩展单元。
type Plugin interface {
	Manifest() Manifest
	Install(*App) error
}

// PluginFunc 将普通安装函数适配为 Plugin。
type PluginFunc struct {
	Info Manifest
	Fn   func(*App) error
}

// Manifest 返回插件清单。
func (p PluginFunc) Manifest() Manifest {
	return p.Info
}

// Install 执行插件安装函数；未提供函数时视为成功。
func (p PluginFunc) Install(app *App) error {
	if p.Fn == nil {
		return nil
	}
	return p.Fn(app)
}

// Hook 是 App 生命周期钩子函数。
type Hook func(context.Context) error

// UsePlugin 立即安装插件，并在成功后记录插件清单。
func (a *App) UsePlugin(plugin Plugin) error {
	if plugin == nil {
		return nil
	}
	manifest := plugin.Manifest()
	if err := plugin.Install(a); err != nil {
		if manifest.Name != "" {
			return fmt.Errorf("anybot: 安装插件 %s 失败: %w", manifest.Name, err)
		}
		return fmt.Errorf("anybot: 安装插件失败: %w", err)
	}
	a.plugins = append(a.plugins, plugin.Manifest())
	return nil
}

// UsePlugins 按传入顺序安装多个插件，遇到错误时停止。
func (a *App) UsePlugins(plugins ...Plugin) error {
	for _, plugin := range plugins {
		if err := a.UsePlugin(plugin); err != nil {
			return err
		}
	}
	return nil
}

// Plugins 返回已成功安装插件的清单副本。
func (a *App) Plugins() []Manifest {
	return append([]Manifest(nil), a.plugins...)
}

// OnStart 注册启动钩子，适配器启动前执行。
func (a *App) OnStart(hook Hook) {
	if hook != nil {
		a.startHooks = append(a.startHooks, hook)
	}
}

// OnReady 注册就绪钩子，事件调度器准备好后、适配器启动前执行。
func (a *App) OnReady(hook Hook) {
	if hook != nil {
		a.readyHooks = append(a.readyHooks, hook)
	}
}

// OnError 注册路由处理错误的回调。
func (a *App) OnError(handler ErrorHandler) {
	if handler != nil {
		a.errorHandlers = append(a.errorHandlers, handler)
	}
}

// OnShutdown 注册关闭钩子，执行顺序与注册顺序相反。
func (a *App) OnShutdown(hook Hook) {
	if hook != nil {
		a.shutdownHooks = append(a.shutdownHooks, hook)
	}
}

// OnStop 是 OnShutdown 的兼容别名。
func (a *App) OnStop(hook Hook) {
	a.OnShutdown(hook)
}

func runHooks(ctx context.Context, hooks []Hook) error {
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		if err := hook(ctx); err != nil {
			return err
		}
	}
	return nil
}

func reverseHooks(hooks []Hook) []Hook {
	out := append([]Hook(nil), hooks...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
