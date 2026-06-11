package core

import (
	"context"
	"sync"
	"time"
)

var (
	defaultMu  sync.Mutex
	defaultApp = New()
)

// Default 返回快捷入口使用的进程级默认 App。
func Default() *App {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultApp
}

// Configure 将配置选项应用到默认 App。
func Configure(opts ...Option) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	for _, opt := range opts {
		opt(defaultApp)
	}
}

// Use 向默认 App 注册全局中间件。
func Use(middleware ...Middleware) {
	Default().Use(middleware...)
}

// OnStart 向默认 App 注册启动钩子。
func OnStart(hook Hook) {
	Default().OnStart(hook)
}

// OnReady 向默认 App 注册就绪钩子。
func OnReady(hook Hook) {
	Default().OnReady(hook)
}

// OnAdapterState 向默认 App 注册适配器状态变化钩子。
func OnAdapterState(hook AdapterStateHook) {
	Default().OnAdapterState(hook)
}

// OnError 向默认 App 注册错误处理回调。
func OnError(handler ErrorHandler) {
	Default().OnError(handler)
}

// OnObserverError 向默认 App 注册观察者错误处理回调。
func OnObserverError(handler ObserverErrorHandler) {
	Default().OnObserverError(handler)
}

// OnShutdown 向默认 App 注册关闭钩子。
func OnShutdown(hook Hook) {
	Default().OnShutdown(hook)
}

// OnStop 是 OnShutdown 的兼容别名。
func OnStop(hook Hook) {
	Default().OnStop(hook)
}

// On 向默认 App 注册通用事件路由。
func On(rules ...Rule) *Route {
	return Default().On(rules...)
}

// Observe 向默认 App 注册事件旁路观察者。
func Observe(rules ...Rule) *Observer {
	return Default().Observe(rules...)
}

// OnMessage 向默认 App 注册消息事件路由。
func OnMessage(rules ...Rule) *Route {
	return Default().OnMessage(rules...)
}

// Command 向默认 App 注册命令路由。
func Command(names ...string) *Route {
	return Default().Command(names...)
}

// Regex 向默认 App 注册文本正则路由。
func Regex(pattern string) *Route {
	return Default().Regex(pattern)
}

// Go 向默认 App 注册生命周期托管的后台任务。
func Go(name string, fn TaskFunc, opts ...TaskOption) {
	Default().Go(name, fn, opts...)
}

// Every 向默认 App 注册生命周期托管的周期任务。
func Every(name string, interval time.Duration, fn TaskFunc, opts ...TaskOption) {
	Default().Every(name, interval, fn, opts...)
}

// GoWhenActionReady 向默认 App 注册等待动作客户端可用后执行的生命周期任务。
func GoWhenActionReady(name string, fn TaskFunc, opts ...TaskOption) {
	Default().GoWhenActionReady(name, fn, opts...)
}

// CurrentAdapterState 返回默认 App 最近一次已知的适配器状态。
func CurrentAdapterState() AdapterState {
	return Default().AdapterState()
}

// WaitActionReady 等待默认 App 的动作客户端可用。
func WaitActionReady(ctx context.Context) error {
	return Default().WaitActionReady(ctx)
}

// Run 启动默认 App；未传 context 时使用 context.Background。
func Run(ctxs ...context.Context) error {
	ctx := context.Background()
	if len(ctxs) > 0 && ctxs[0] != nil {
		ctx = ctxs[0]
	}
	return Default().Run(ctx)
}

// ResetDefault 使用新配置替换默认 App，主要用于测试和极小工具。
func ResetDefault(opts ...Option) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultApp = New(opts...)
}
