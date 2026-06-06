package sdk

import (
	"context"
	"log/slog"
	"time"

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/sdk/message"
)

// Context 是成熟插件的安装上下文，暴露插件应使用的运行时能力。
type Context struct {
	app      *App
	router   *core.Router
	manifest Manifest
}

// NewContext 创建插件安装上下文，主要供宿主或测试使用。
func NewContext(app *App, manifest Manifest) *Context {
	ctx := &Context{app: app, manifest: manifest}
	if app != nil {
		ctx.router = app.Group()
	}
	return ctx
}

// App 返回底层 App。常规插件应优先使用 Context 上的窄接口方法；这里主要作为高级逃生口。
func (c *Context) App() *App {
	if c == nil {
		return nil
	}
	return c.app
}

// Manifest 返回当前插件清单。
func (c *Context) Manifest() Manifest {
	if c == nil {
		return Manifest{}
	}
	return c.manifest
}

// Logger 返回宿主日志器。
func (c *Context) Logger() *slog.Logger {
	if c == nil || c.app == nil {
		return slog.Default()
	}
	logger := c.app.Logger()
	if c.manifest.Name == "" {
		return logger
	}
	return logger.With("plugin", c.manifest.Name)
}

// Store 返回宿主会话存储。
func (c *Context) Store() Store {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.Store()
}

// Session 返回当前插件命名空间下的会话存储视图。
func (c *Context) Session(event *EventContext) *Session {
	if event == nil {
		return c.SessionBy("session")
	}
	return c.SessionBy(event.ConversationID())
}

// UserSession 返回当前插件命名空间下的用户存储视图。
func (c *Context) UserSession(event *EventContext) *Session {
	if event == nil {
		return c.SessionBy("session:user")
	}
	return c.SessionBy(event.UserSession().Key())
}

// GroupSession 返回当前插件命名空间下的群或频道存储视图。
func (c *Context) GroupSession(event *EventContext) *Session {
	if event == nil {
		return c.SessionBy("session:group")
	}
	return c.SessionBy(event.GroupSession().Key())
}

// SessionBy 返回当前插件命名空间下的自定义存储视图。
func (c *Context) SessionBy(key string) *Session {
	if key == "" {
		key = "session"
	}
	return NewSession(c.Store(), scopedName(c.manifest.Name, key))
}

// Client 返回当前动作客户端。
func (c *Context) Client() ActionClient {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.Client()
}

// Send 使用当前动作客户端发送消息。
func (c *Context) Send(ctx context.Context, target ReplyTarget, chain message.Chain) (MessageReceipt, error) {
	if c == nil || c.app == nil || c.app.Client() == nil {
		return MessageReceipt{}, ErrActionUnavailable
	}
	return c.app.Client().Send(ctx, target, chain)
}

// SendText 使用当前动作客户端发送纯文本消息。
func (c *Context) SendText(ctx context.Context, target ReplyTarget, text string) (MessageReceipt, error) {
	return c.Send(ctx, target, message.New(message.Text(text)))
}

// Use 注册插件级中间件，只作用于当前插件通过 Context 注册的后续路由。
func (c *Context) Use(middleware ...Middleware) {
	if c != nil && c.router != nil {
		c.router.Use(middleware...)
	}
}

// UseGlobal 注册宿主级全局中间件。普通插件应优先使用 Use；只有安全策略、全局限流等宿主策略才应使用此方法。
func (c *Context) UseGlobal(middleware ...Middleware) {
	if c != nil && c.app != nil {
		c.app.Use(middleware...)
	}
}

// On 注册通用事件路由。
func (c *Context) On(rules ...Rule) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return c.router.On(rules...)
}

// OnMessage 注册消息事件路由。
func (c *Context) OnMessage(rules ...Rule) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return c.router.OnMessage(rules...)
}

// Command 注册命令路由。
func (c *Context) Command(names ...string) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return c.router.Command(names...)
}

// Observe 注册事件旁路观察者。
func (c *Context) Observe(rules ...Rule) *Observer {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.Observe(rules...)
}

// Go 注册生命周期托管的后台任务。
func (c *Context) Go(name string, fn TaskFunc, opts ...TaskOption) {
	if c != nil && c.app != nil {
		c.app.Go(scopedName(c.manifest.Name, name), fn, opts...)
	}
}

// Every 注册生命周期托管的周期任务。
func (c *Context) Every(name string, interval time.Duration, fn TaskFunc, opts ...TaskOption) {
	if c != nil && c.app != nil {
		c.app.Every(scopedName(c.manifest.Name, name), interval, fn, opts...)
	}
}

// OnStart 注册启动钩子，适配器启动前执行。
func (c *Context) OnStart(hook Hook) {
	if c != nil && c.app != nil {
		c.app.OnStart(hook)
	}
}

// OnReady 注册就绪钩子，事件调度器准备好后、适配器启动前执行。
func (c *Context) OnReady(hook Hook) {
	if c != nil && c.app != nil {
		c.app.OnReady(hook)
	}
}

// OnError 注册全局路由错误回调，会影响整个宿主；插件局部错误优先使用路由中间件处理。
func (c *Context) OnError(handler ErrorHandler) {
	if c != nil && c.app != nil {
		c.app.OnError(handler)
	}
}

// OnShutdown 注册关闭钩子。
func (c *Context) OnShutdown(hook Hook) {
	if c != nil && c.app != nil {
		c.app.OnShutdown(hook)
	}
}

// WaitActionReady 等待动作客户端可用。
func (c *Context) WaitActionReady(ctx context.Context) error {
	if c == nil || c.app == nil {
		return core.ErrActionUnavailable
	}
	return c.app.WaitActionReady(ctx)
}

// OnAdapterState 注册动作通道状态钩子。
func (c *Context) OnAdapterState(hook AdapterStateHook) {
	if c != nil && c.app != nil {
		c.app.OnAdapterState(hook)
	}
}

func scopedName(pluginName, name string) string {
	if pluginName == "" {
		if name == "" {
			return "plugin"
		}
		return name
	}
	if name == "" {
		return pluginName
	}
	return pluginName + "." + name
}
