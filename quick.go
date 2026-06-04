package anybot

import (
	"context"
	"sync"
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

// UsePlugin 向默认 App 安装插件。
func UsePlugin(plugin Plugin) error {
	return Default().UsePlugin(plugin)
}

// UsePlugins 向默认 App 按顺序安装多个插件。
func UsePlugins(plugins ...Plugin) error {
	return Default().UsePlugins(plugins...)
}

// OnStart 向默认 App 注册启动钩子。
func OnStart(hook Hook) {
	Default().OnStart(hook)
}

// OnReady 向默认 App 注册就绪钩子。
func OnReady(hook Hook) {
	Default().OnReady(hook)
}

// OnError 向默认 App 注册错误处理回调。
func OnError(handler ErrorHandler) {
	Default().OnError(handler)
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
