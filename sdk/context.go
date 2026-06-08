package sdk

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/sdk/message"
)

// ErrGlobalMiddlewareUnavailable 表示宿主没有授予插件注册全局中间件的能力。
var ErrGlobalMiddlewareUnavailable = errors.New("anybot: global middleware is not available to this plugin")

// Context 是成熟插件的安装上下文，暴露插件应使用的运行时能力。
type Context struct {
	app      *App
	router   *core.Router
	manifest Manifest
	env      Environment
}

// NewContext 创建插件安装上下文，主要供运行框架或测试使用。
func NewContext(app *App, manifest Manifest, opts ...InstallOption) *Context {
	ctx := &Context{app: app, manifest: manifest}
	for _, opt := range opts {
		if opt != nil {
			opt(&ctx.env)
		}
	}
	if app != nil {
		ctx.router = app.Group()
	}
	return ctx
}

// Manifest 返回当前插件清单。
func (c *Context) Manifest() Manifest {
	if c == nil {
		return Manifest{}
	}
	return c.manifest
}

// PluginID 返回当前插件在本地机器人目录中的稳定安装 ID。
func (c *Context) PluginID() string {
	if c == nil {
		return ""
	}
	return c.env.PluginID
}

// Logger 返回框架日志器。
func (c *Context) Logger() *slog.Logger {
	if c == nil || c.app == nil {
		return slog.Default()
	}
	logger := c.app.Logger()
	id := c.PluginID()
	if id == "" {
		return logger
	}
	return logger.With("plugin_id", id, "plugin_name", c.manifest.Name)
}

// Store 返回框架会话存储。
func (c *Context) Store() Store {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.Store()
}

// Session 返回当前 PluginID 命名空间下的会话存储视图。
func (c *Context) Session(event *EventContext) *Session {
	if event == nil {
		return c.SessionBy("session")
	}
	return c.SessionBy(event.ConversationID())
}

// UserSession 返回当前 PluginID 命名空间下的用户存储视图。
func (c *Context) UserSession(event *EventContext) *Session {
	if event == nil {
		return c.SessionBy("session:user")
	}
	return c.SessionBy(event.UserSession().Key())
}

// GroupSession 返回当前 PluginID 命名空间下的群或频道存储视图。
func (c *Context) GroupSession(event *EventContext) *Session {
	if event == nil {
		return c.SessionBy("session:group")
	}
	return c.SessionBy(event.GroupSession().Key())
}

// SessionBy 返回当前 PluginID 命名空间下的自定义存储视图。
func (c *Context) SessionBy(key string) *Session {
	if key == "" {
		key = "session"
	}
	return NewSession(c.Store(), scopedName(c.PluginID(), key))
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

// UseGlobal 注册框架级全局中间件。只有运行框架显式授权的内置策略插件才应使用。
func (c *Context) UseGlobal(middleware ...Middleware) error {
	if c == nil || c.app == nil {
		return ErrGlobalMiddlewareUnavailable
	}
	if !c.env.AllowGlobalMiddleware {
		return ErrGlobalMiddlewareUnavailable
	}
	c.app.Use(middleware...)
	return nil
}

// On 注册通用事件路由。
func (c *Context) On(rules ...Rule) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return newRoute(c, c.router.On(rules...))
}

// OnMessage 注册消息事件路由。
func (c *Context) OnMessage(rules ...Rule) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return newRoute(c, c.router.OnMessage(rules...))
}

// Command 注册命令路由。
func (c *Context) Command(names ...string) *Route {
	if c == nil || c.router == nil {
		return nil
	}
	return newRoute(c, c.router.Command(names...))
}

// Observe 注册事件旁路观察者。
func (c *Context) Observe(rules ...Rule) *Observer {
	if c == nil || c.app == nil {
		return nil
	}
	return newObserver(c, c.app.Observe(rules...))
}

// Go 注册生命周期托管的后台任务。
func (c *Context) Go(name string, fn TaskFunc, opts ...TaskOption) {
	if c != nil && c.app != nil {
		c.app.Go(scopedName(c.PluginID(), name), fn, opts...)
	}
}

// Every 注册生命周期托管的周期任务。
func (c *Context) Every(name string, interval time.Duration, fn TaskFunc, opts ...TaskOption) {
	if c != nil && c.app != nil {
		c.app.Every(scopedName(c.PluginID(), name), interval, fn, opts...)
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

// Route 是插件 ID 命名空间下的一条事件路由。
type Route struct {
	ctx   *Context
	route *core.Route
}

func newRoute(ctx *Context, route *core.Route) *Route {
	if route == nil {
		return nil
	}
	return &Route{ctx: ctx, route: route}
}

// Name 设置路由名称，名称会自动落入当前插件命名空间。
func (r *Route) Name(name string) *Route {
	if r != nil && r.route != nil {
		r.route.Name(scopedName(r.pluginName(), name))
	}
	return r
}

// Priority 设置路由优先级；数值越大越先执行，同优先级保持注册顺序。
func (r *Route) Priority(priority int) *Route {
	if r != nil && r.route != nil {
		r.route.Priority(priority)
	}
	return r
}

// Use 追加只作用于当前路由的中间件。
func (r *Route) Use(middleware ...Middleware) *Route {
	if r != nil && r.route != nil {
		r.route.Use(middleware...)
	}
	return r
}

// Handle 设置路由处理函数。
func (r *Route) Handle(handler Handler) *Route {
	if r != nil && r.route != nil {
		r.route.Handle(handler)
	}
	return r
}

func (r *Route) pluginName() string {
	if r == nil || r.ctx == nil {
		return ""
	}
	return r.ctx.PluginID()
}

// Observer 是插件 ID 命名空间下的一条旁路事件观察规则。
type Observer struct {
	ctx      *Context
	observer *core.Observer
}

func newObserver(ctx *Context, observer *core.Observer) *Observer {
	if observer == nil {
		return nil
	}
	return &Observer{ctx: ctx, observer: observer}
}

// Name 设置观察者名称，名称会自动落入当前插件命名空间。
func (o *Observer) Name(name string) *Observer {
	if o != nil && o.observer != nil {
		o.observer.Name(scopedName(o.pluginName(), name))
	}
	return o
}

// Handle 设置观察者处理函数。
func (o *Observer) Handle(handler ObserverHandler) *Observer {
	if o != nil && o.observer != nil {
		o.observer.Handle(handler)
	}
	return o
}

func (o *Observer) pluginName() string {
	if o == nil || o.ctx == nil {
		return ""
	}
	return o.ctx.PluginID()
}

func scopedName(pluginName, name string) string {
	pluginName = strings.TrimSpace(pluginName)
	name = strings.TrimSpace(name)
	if pluginName == "" {
		if name == "" {
			return "plugin"
		}
		return name
	}
	if name == "" {
		return pluginName
	}
	if name == pluginName || strings.HasPrefix(name, pluginName+".") {
		return name
	}
	return pluginName + "." + name
}
