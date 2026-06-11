package core

import (
	"context"
)

// Hook 是 App 生命周期钩子函数。
type Hook func(context.Context) error

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
